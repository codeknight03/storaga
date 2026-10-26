// Package fops provides the core file operations for an append-only record log.
package fops

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os"
	"time"
)

// MAXLENKEYVAL is the max size of a key or a value: 512 MiB (see header in models.go).
const MAXLENKEYVAL = 512 * 1024 * 1024

// serialize encodes r into its on-disk layout.
// No error: it only appends already-typed values to an in-memory buffer.
func (r Record) serialize() []byte {
	buf := make([]byte, 0, 4+8+4+4+r.header.keyLen+r.header.valLen)

	// header; crc is a 4-byte slot filled in by Put after checksumming the rest
	buf = binary.LittleEndian.AppendUint32(buf, r.header.crc)
	buf = binary.LittleEndian.AppendUint64(buf, r.header.ts)
	buf = binary.LittleEndian.AppendUint32(buf, r.header.keyLen)
	buf = binary.LittleEndian.AppendUint32(buf, r.header.valLen)

	// payload
	buf = append(buf, r.key...)

	buf = append(buf, r.val...)

	return buf

}

// deserialize decodes one record from buf.
// Read path: draft, not used yet — out of scope for the append phase.
func deserialize(buf []byte) (Record, error) {

	var r Record

	r.header.crc = binary.LittleEndian.Uint32(buf)
	r.header.ts = binary.LittleEndian.Uint64(buf[4:])
	r.header.keyLen = binary.LittleEndian.Uint32(buf[12:])
	r.header.valLen = binary.LittleEndian.Uint32(buf[16:])
	r.key = buf[20:(20 + r.header.keyLen)]
	r.val = buf[(20 + r.header.keyLen):(20 + r.header.keyLen + r.header.valLen)]

	return r, nil

}

// Put builds a record from key and val and appends it to fd, fsyncing before
// it returns.
//
// Decision: who creates/opens the store file.
// Options:
//   - This package owns a global file and creates it on startup
//     (like Postgres CREATE DATABASE creating files underneath).
//   - The caller (main) opens the file and passes the descriptor in.
//
// Why: the caller. This package only does record ops on a given file;
// main decides which file that is.
// Revisit if: TBD.
//
// Decision: key/val as slices, not fixed 512-byte arrays.
// Why: fixed arrays waste space on every record; slices let the runtime
// allocate only what's needed.
// Open: a caller can still hand us an unbounded byte slic for key and value,
// so memory use per Put is effectively unbounded — revisit later.
func Put(key []byte, val []byte, fd os.File) error {

	var r Record

	// validate everything before using
	if len(key) > MAXLENKEYVAL {
		return fmt.Errorf("Key too long")
	}

	if len(val) > MAXLENKEYVAL {
		return fmt.Errorf("Val too long")
	}

	if fdInfo, err := fd.Stat(); err != nil || fdInfo.Mode()&fs.ModeAppend != 0 {
		return fmt.Errorf("Invalid file descriptor: append not enabled or not a regular file")
	}

	// Build the record.
	// The uint32 casts are safe: both lengths were checked against MAXLENKEYVAL (512 MiB) above.
	// Open: ts is taken before encoding/writing, so it's slightly earlier than
	//       the actual write. Acceptable?
	r.header.keyLen = uint32(len(key))
	r.header.valLen = uint32(len(val))
	r.header.ts = uint64(time.Now().UnixNano())
	r.key = key
	r.val = val

	// Decision: what the CRC covers.
	// Why: header + payload, not just the payload, so a corrupted timestamp
	//      or length field is detected too.
	//
	// Encode with the crc slot zeroed, checksum everything after the slot,
	// then put the crc in front.
	data := r.serialize()
	r.header.crc = crc32.Checksum(data[4:], crc32.IEEETable)

	crcBuf := make([]byte, 0, 4)
	crcBuf = binary.LittleEndian.AppendUint32(crcBuf, r.header.crc)
	data = append(crcBuf, data[4:]...)

	//Get last good offset before we touch the file
	fdInfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("Failed to get file offset prewrite: %s", fd.Name())

	}

	lastGood := fdInfo.Size()

	// Append (relies on fd being opened with O_APPEND), then fsync so the
	// record is durable before Put returns nil.
	_, err = fd.Write(data)

	if err != nil {
		//Decision: How to handle a write error without causing unreadable records after it ?
		//Options: 1. Break the writer for good and don't accept writes any more
		//         2. Go to the lastGood point and leave the FD there.
		//Why: Choosing 2 is obvious here instead of breaking the world we recover from what is
		//     recoverable but the problem is should lastGood have been a global state we controlled
		//     in this package that would have made sense if the file was this package's responsibility
		//     since its not we just Seek() the current offset of the given fd and leave it as is if
		//     we incur a write error
		fd.Truncate(lastGood)

		return fmt.Errorf("Failed to write to file: %s", fd.Name())

	}
	err = fd.Sync()
	if err != nil {
		return fmt.Errorf("Failed to sync file to storage: %s", fd.Name())
	}

	return nil
}
