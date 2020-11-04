package service

import (
	"errors"
	"github.com/ToolPackage/fse/utils"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
)

type FileStorage struct {
	storagePath string
	files       map[string]*File
	dataFiles   []*SequentialFile
	cache       PartitionCache
}

type File struct {
	fileName    string // 128
	fileSize    uint32 // 8
	contentType string // 32
	createdAt   int64  // 8
	partitions  Partitions
}

//PartitionId = sequential file id + file chunk id
type PartitionId uint32
type Partitions []PartitionId

const maxPartitionNum = 0xffff - 1 // 65535, 2Bytes

func NewFileStorage() *FileStorage {
	storagePath := getStoragePath()

	// scan storage path and open all sequential files
	dataFilePath := path.Join(storagePath, "datafiles")
	fileNames, err := filepath.Glob(dataFilePath)
	if err != nil {
		panic(err)
	}

	dataFiles := make([]*SequentialFile, len(fileNames))
	for _, fileName := range fileNames {
		id, err := strconv.ParseInt(fileName, 10, 16)
		if err != nil {
			panic(err)
		}

		dataFile, err := NewSequentialFile(path.Join(dataFilePath, fileName),
			MaxFileChunkDataSize, MaxFileChunkNum)
		dataFiles[id] = dataFile
	}

	files := readMetadataFile(storagePath)

	return &FileStorage{
		storagePath: storagePath,
		files:       files,
		dataFiles:   dataFiles,
	}
}

func readMetadataFile(storagePath string) map[string]*File {
	// TODO: bug
	defer func() {
		if err := recover(); err != nil && err == io.EOF {
			err = nil
		}
	}()

	// read file metadata
	metadataFile := NewEntrySequenceFile(path.Join(storagePath, "metadata.esf"), ReadMode)

	var files = make(map[string]*File)
	for true {
		file := &File{}
		file.fileName = string(metadataFile.ReadEntry())
		file.fileSize = utils.ConvertByteToUint32(metadataFile.ReadEntry(), 0)
		file.contentType = string(metadataFile.ReadEntry())
		file.createdAt = utils.ConvertByteToInt64(metadataFile.ReadEntry(), 0)
		partitionNum := utils.ConvertByteToUint16(metadataFile.ReadEntry(), 0)
		partitions := make([]PartitionId, partitionNum)
		for i := uint16(0); i < partitionNum; i++ {
			partitions[i] = PartitionId(utils.ConvertByteToUint32(metadataFile.ReadEntry(), 0))
		}
		files[file.fileName] = file
	}

	return files
}

func getStoragePath() string {
	return filepath.Join(getUserHomeDir(), ".fse")
}

func getUserHomeDir() string {
	home := os.Getenv("HOME")
	if home != "" {
		return home
	}

	if runtime.GOOS == "windows" {
		home = os.Getenv("USERPROFILE")
		if home != "" {
			return home
		}

		home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home != "" {
			return home
		}
	}

	panic("could not detect home directory")
}

func (fs *FileStorage) OpenStream(file *File) io.Reader {
	return newFileDataReader(fs, file.partitions)
}

func (fs *FileStorage) SaveFileData(input io.Reader) (Partitions, error) {
	return nil, nil
}

func (fs *FileStorage) GetChunk(id PartitionId) (*FileChunk, error) {
	return nil, nil
}

type FileDataReader struct {
	fs               *FileStorage
	partitions       Partitions
	nextPartitionIdx int
	currentChunk     *FileChunk
	chunkReadOffset  int
}

func newFileDataReader(fs *FileStorage, partitions Partitions) *FileDataReader {
	return &FileDataReader{
		fs:               fs,
		partitions:       partitions,
		nextPartitionIdx: -1,
		currentChunk:     nil,
		chunkReadOffset:  0,
	}
}

func (r *FileDataReader) Read(p []byte) (n int, err error) {
	chunk, err := r.getAvailableChunk()
	if err != nil {
		return
	}

	availableBytes := len(chunk.content) - r.chunkReadOffset

	n = utils.Min(availableBytes, len(p))
	result := copy(p, chunk.content[r.chunkReadOffset:r.chunkReadOffset+n])
	if n != result {
		n = result
		err = errors.New("copy chunk data error")
	}
	r.chunkReadOffset += n
	return
}

// return nil when all chunks are consumed or chunk couldn't be load by file storage
func (r *FileDataReader) getAvailableChunk() (*FileChunk, error) {
	var err error
	if r.currentChunk == nil || r.chunkReadOffset >= len(r.currentChunk.content) {
		// get next chunk
		r.nextPartitionIdx++
		if r.nextPartitionIdx >= len(r.partitions) {
			err = EndOfPartitionStreamError
		} else {
			r.currentChunk, err = r.fs.GetChunk(r.partitions[r.nextPartitionIdx])
		}
	}
	return r.currentChunk, err
}
