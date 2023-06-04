package logger

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var bpool = newBufferPool(500)

type bufferPool struct {
	p     sync.Pool
	count int64
	size  int
}

func newBufferPool(size int) *bufferPool {
	p := &bufferPool{}
	p.p.New = func() interface{} {
		return bytes.NewBuffer(nil)
	}

	p.size = size

	return p
}

func putBuffer(b *bytes.Buffer) {
	bpool.p.Put(b)
	atomic.AddInt64(&bpool.count, -1)

	return
}

func getBuffer() *bytes.Buffer {
	for {
		c := atomic.LoadInt64(&bpool.count)
		if c > int64(bpool.size) {
			return nil
		}

		if atomic.CompareAndSwapInt64(&bpool.count, c, c+1) {
			b := bpool.p.Get().(*bytes.Buffer)
			b.Reset()

			return b
		}
	}
}

/* }}} */

// RollingFile : Defination of rolling
type RollingFile struct {
	mu sync.Mutex

	closed    bool
	exit      chan struct{}
	syncFlush chan struct{}

	file       *os.File
	current    *bytes.Buffer
	fullBuffer chan *bytes.Buffer

	basePath string
	filePath string
	fileFrag string
	fileExt  string

	rollMutex sync.RWMutex
	rolling   RollingFormat
}

// Errors
var (
	ErrClosedRollingFile = errors.New("rolling file is closed")
	ErrBuffer            = errors.New("buffer exceeds the limit")
)

// RollingFormat : Type hinting
type RollingFormat string

// RollingFormats
const (
	MonthlyRolling  RollingFormat = "200601"
	DailyRolling                  = "20060102"
	HourlyRolling                 = "2006010215"
	MinutelyRolling               = "200601021504"
	SecondlyRolling               = "20060102150405"
)

const (
	logPageCacheByteSize = 4096
	logPageNumber        = 2
	defaultFileExt       = "log"
)

// SetRolling : Set rolling format
func (r *RollingFile) SetRolling(fmt RollingFormat) {
	r.rollMutex.Lock()
	r.rolling = fmt
	r.rollMutex.Unlock()

	return
}

/* {{{ [roll] */
func (r *RollingFile) roll() error {
	r.rollMutex.RLock()
	roll := r.rolling
	now := time.Now()
	r.rollMutex.RUnlock()
	suffix := now.Format(string(roll))
	if r.file != nil {
		if suffix == r.fileFrag {
			return nil
		}

		r.file.Close()
		r.file = nil
	}

	r.fileFrag = suffix
	dir, filename := filepath.Split(r.basePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}
	}

	if r.fileFrag == "" {
		r.filePath = filepath.Join(dir, filename+r.fileExt)
	} else {
		tDir := dir
		tFilename := dir
		switch r.rolling {
		case MonthlyRolling:
			tDir = fmt.Sprintf(
				"%s/%04d",
				dir,
				now.Year(),
			)
			tFilename = fmt.Sprintf(
				"%s_%02d.%s",
				filename,
				now.Month(),
				r.fileExt,
			)
		case DailyRolling:
			tDir = fmt.Sprintf(
				"%s/%04d%02d",
				dir,
				now.Year(),
				now.Month(),
			)
			tFilename = fmt.Sprintf(
				"%s_%02d.%s",
				filename,
				now.Day(),
				r.fileExt,
			)
		case HourlyRolling:
			tDir = fmt.Sprintf(
				"%s/%04d%02d/%02d",
				dir,
				now.Year(),
				now.Month(),
				now.Day(),
			)
			tFilename = fmt.Sprintf(
				"%s_%02d.%s",
				filename,
				now.Hour(),
				r.fileExt,
			)
		case MinutelyRolling:
			tDir = fmt.Sprintf(
				"%s/%04d%02d/%02d/%02d",
				dir,
				now.Year(),
				now.Month(),
				now.Day(),
				now.Hour(),
			)
			tFilename = fmt.Sprintf(
				"%s_%02d.%s",
				filename,
				now.Minute(),
				r.fileExt,
			)
		case SecondlyRolling:
			tDir = fmt.Sprintf(
				"%s/%04d%02d/%02d/%02d/%02d",
				dir,
				now.Year(),
				now.Month(),
				now.Day(),
				now.Hour(),
				now.Minute(),
			)
			tFilename = fmt.Sprintf(
				"%s_%02d.%s",
				filename,
				now.Second(),
				r.fileExt,
			)
		}

		r.filePath = filepath.Join(tDir, tFilename)
	}

	// Make sub dir again
	dir, filename = filepath.Split(r.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(r.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	r.file = f

	return nil
}

/* }}} */

/* {{{ [createSymLink] */
func (r *RollingFile) createSymLink(real, sym string) {
	if _, err := os.Lstat(sym); err == nil {
		os.Remove(sym)
	}

	os.Symlink(real, sym)
}

/* }}} */

// Close syncer file
func (r *RollingFile) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}

	r.closed = true
	r.mu.Unlock()
	close(r.exit)

	return nil
}

func (r *RollingFile) Write(b []byte) (n int, err error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return 0, ErrClosedRollingFile
	}

	if r.current == nil {
		r.current = getBuffer()
		if r.current == nil {
			r.mu.Unlock()
			return 0, ErrBuffer
		}
	}

	n, err = r.current.Write(b)
	if r.current.Len() > logPageCacheByteSize {
		buf := r.current
		r.current = nil
		r.mu.Unlock()
		r.fullBuffer <- buf
	} else {
		r.mu.Unlock()
	}

	return
}

// Sync buffered data to writer
func (r *RollingFile) Sync() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return ErrClosedRollingFile
	}

	r.mu.Unlock()
	r.syncFlush <- struct{}{}
	<-r.syncFlush

	return nil
}

/* {{{ [writeBuffer] */
func (r *RollingFile) writeBuffer(buff *bytes.Buffer) {
	if buff != nil && buff.Len() > 0 {
		if err := r.roll(); err != nil {
		} else {
			buff.WriteTo(r.file)
		}
	}
}

/* }}} */

// flushRoutine : ...
func (r *RollingFile) flushRoutine() {
	t := time.NewTicker(500 * time.Millisecond)

	flush := func() {
		readyLen := len(r.fullBuffer)
		for i := 0; i < readyLen; i++ {
			buff := <-r.fullBuffer
			r.writeBuffer(buff)
			putBuffer(buff)
		}

		if r.current != nil {
			r.writeBuffer(r.current)
			putBuffer(r.current)
		}

		r.current = nil
		if r.file != nil {
			r.file.Sync()
		}
	}

	//FIXME better solution ?
	defer func() {
		t.Stop()
		flush()
		if f := r.file; f != nil {
			r.file = nil
			f.Close()
		}
	}()

	for {
		select {
		case <-r.syncFlush:
			r.mu.Lock()
			flush()
			r.mu.Unlock()
			r.syncFlush <- struct{}{}
		case buff := <-r.fullBuffer:
			r.writeBuffer(buff)
			putBuffer(buff)
		case <-t.C:
			r.mu.Lock()
			if len(r.fullBuffer) != 0 {
				r.mu.Unlock()
				continue
			}

			buff := r.current
			if buff == nil {
				r.mu.Unlock()
				continue
			}

			r.current = nil
			r.mu.Unlock()
			r.writeBuffer(buff)
			putBuffer(buff)
		case <-r.exit:
			return
		}
	}
}

// NewRollingFile create new rolling
func NewRollingFile(basePath string, rolling RollingFormat) (*RollingFile, error) {
	basePath = strings.TrimSuffix(basePath, "."+defaultFileExt)
	if _, file := filepath.Split(basePath); file == "" {
		return nil, fmt.Errorf("invalid base-path = %s, file name is required", basePath)
	}

	r := &RollingFile{
		basePath:   basePath,
		rolling:    rolling,
		exit:       make(chan struct{}),
		syncFlush:  make(chan struct{}),
		closed:     false,
		fullBuffer: make(chan *bytes.Buffer, logPageNumber+1),
		current:    getBuffer(),
		fileExt:    defaultFileExt,
	}
	// fill ready buffer
	go r.flushRoutine()

	return r, nil
}
