package sdk

import (
	"io"
	"math"
	"strconv"

	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/encryption"
	"github.com/0chain/gosdk/zboxcore/zboxutil"
	"github.com/klauspost/reedsolomon"
)

type ChunkedUploadChunkReader interface {
	// Next read, encode and encrypt next chunk
	Next() (*ChunkData, error)

	// Read read, encode and encrypt all bytes
	Read(buf []byte) ([][]byte, error)
}

// chunkedUploadChunkReader read chunk bytes from io.Reader. see detail on https://github.com/0chain/blobber/wiki/Protocols#what-is-fixedmerkletree
type chunkedUploadChunkReader struct {
	fileReader io.Reader

	//size total size of source. 0 means we don't it
	size int64
	// readSize total read size from source
	readSize int64

	// chunkSize chunk size with encryption header
	chunkSize int64

	// chunkHeaderSize encrypt header size
	chunkHeaderSize int64
	// chunkDataSize data size without encryption header in a chunk. It is same as ChunkSize if EncryptOnUpload is false
	chunkDataSize int64

	// chunkDataSizePerRead total size should be read from original io.Reader. It is DataSize * DataShards.
	chunkDataSizePerRead int64

	// nextChunkIndex next index for reading
	nextChunkIndex int

	dataShards int

	// encryptOnUpload enccrypt data on upload
	encryptOnUpload bool

	uploadMask zboxutil.Uint128
	// erasureEncoder erasuer encoder
	erasureEncoder reedsolomon.Encoder
	// encscheme encryption scheme
	encscheme encryption.EncryptionScheme
	// hasher to calculate actual file hash, validation root and fixed merkle root
	hasher Hasher
}

// createChunkReader create ChunkReader instance
func createChunkReader(fileReader io.Reader, size, chunkSize int64, dataShards int, encryptOnUpload bool, uploadMask zboxutil.Uint128, erasureEncoder reedsolomon.Encoder, encscheme encryption.EncryptionScheme, hasher Hasher) (ChunkedUploadChunkReader, error) {

	if chunkSize <= 0 {
		return nil, errors.Throw(constants.ErrInvalidParameter, "chunkSize: "+strconv.FormatInt(chunkSize, 10))
	}

	if dataShards <= 0 {
		return nil, errors.Throw(constants.ErrInvalidParameter, "dataShards: "+strconv.Itoa(dataShards))
	}

	if erasureEncoder == nil {
		return nil, errors.Throw(constants.ErrInvalidParameter, "erasureEncoder")
	}

	if hasher == nil {
		return nil, errors.Throw(constants.ErrInvalidParameter, "hasher")
	}

	r := &chunkedUploadChunkReader{
		fileReader:      fileReader,
		size:            size,
		chunkSize:       chunkSize,
		nextChunkIndex:  0,
		dataShards:      dataShards,
		encryptOnUpload: encryptOnUpload,
		uploadMask:      uploadMask,
		erasureEncoder:  erasureEncoder,
		encscheme:       encscheme,
		hasher:          hasher,
	}

	if r.encryptOnUpload {
		//additional 16 bytes to save encrypted data
		r.chunkHeaderSize = EncryptedDataPaddingSize + EncryptionHeaderSize
		r.chunkDataSize = chunkSize - r.chunkHeaderSize
	} else {
		r.chunkDataSize = chunkSize
	}

	r.chunkDataSizePerRead = r.chunkDataSize * int64(dataShards)

	return r, nil
}

// ChunkData data of a chunk
type ChunkData struct {
	// Index current index of chunks
	Index int
	// IsFinal last chunk or not
	IsFinal bool

	// ReadSize total size read from original reader (un-encoded, un-encrypted)
	ReadSize int64
	// FragmentSize fragment size for a blobber (un-encrypted)
	FragmentSize int64
	// Fragments data shared for bloobers
	Fragments [][]byte
}

// func (r *chunkReader) GetChunkDataSize() int64 {
// 	if r == nil {
// 		return 0
// 	}
// 	return r.chunkDataSize
// }

// Next read next chunks for blobbers
func (r *chunkedUploadChunkReader) Next() (*ChunkData, error) {

	if r == nil {
		return nil, errors.Throw(constants.ErrInvalidParameter, "r")
	}

	chunk := &ChunkData{
		Index:   r.nextChunkIndex,
		IsFinal: false,

		ReadSize:     0,
		FragmentSize: 0,
	}

	chunkBytes := make([]byte, r.chunkDataSizePerRead)
	readLen, err := r.fileReader.Read(chunkBytes)
	if err != nil {

		if !errors.Is(err, io.EOF) {
			return nil, err
		}

		//all bytes are read
		chunk.IsFinal = true
	}

	if readLen == 0 {
		chunk.IsFinal = true
		return chunk, nil
	}

	chunk.FragmentSize = int64(math.Ceil(float64(readLen)/float64(r.dataShards))) + r.chunkHeaderSize

	if readLen < int(r.chunkDataSizePerRead) {
		chunkBytes = chunkBytes[:readLen]
		chunk.IsFinal = true
	}

	chunk.ReadSize = int64(readLen)
	r.readSize += chunk.ReadSize
	if r.size > 0 {
		if r.readSize >= r.size {
			chunk.IsFinal = true
		}
	}

	err = r.hasher.WriteToFile(chunkBytes)
	if err != nil {
		return chunk, err
	}

	fragments, err := r.erasureEncoder.Split(chunkBytes)
	if err != nil {
		return nil, err
	}

	err = r.erasureEncoder.Encode(fragments)
	if err != nil {
		return nil, err
	}

	var pos uint64
	if r.encryptOnUpload {
		for i := r.uploadMask; !i.Equals64(0); i = i.And(zboxutil.NewUint128(1).Lsh(pos).Not()) {
			pos = uint64(i.TrailingZeros())
			encMsg, err := r.encscheme.Encrypt(fragments[pos])
			if err != nil {
				return nil, err
			}
			fragments[pos] = make([]byte, len(encMsg.EncryptedData)+EncryptionHeaderSize)
			n := copy(fragments[pos], encMsg.MessageChecksum+encMsg.OverallChecksum)
			copy(fragments[pos][n:], encMsg.EncryptedData)
		}
	}

	chunk.Fragments = fragments
	r.nextChunkIndex++
	return chunk, nil
}

// Read read, encode and encrypt all bytes
func (r *chunkedUploadChunkReader) Read(buf []byte) ([][]byte, error) {

	if buf == nil {
		return nil, nil
	}

	if r == nil {
		return nil, errors.Throw(constants.ErrInvalidParameter, "r")
	}

	fragments, err := r.erasureEncoder.Split(buf)
	if err != nil {
		return nil, err
	}

	err = r.erasureEncoder.Encode(fragments)
	if err != nil {
		return nil, err
	}

	var pos uint64
	if r.encryptOnUpload {
		for i := r.uploadMask; !i.Equals64(0); i = i.And(zboxutil.NewUint128(1).Lsh(pos).Not()) {
			pos = uint64(i.TrailingZeros())
			encMsg, err := r.encscheme.Encrypt(fragments[pos])
			if err != nil {
				return nil, err
			}
			fragments[pos] = make([]byte, len(encMsg.EncryptedData)+EncryptionHeaderSize)
			n := copy(fragments[pos], encMsg.MessageChecksum+encMsg.OverallChecksum)
			copy(fragments[pos][n:], encMsg.EncryptedData)
		}
	}

	return fragments, nil
}
