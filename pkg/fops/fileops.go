// exports the core file ops needed for a log append store
package fops

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"time"
)

const MAXLENKEYVAL = 512 * 1024 * 1024

// Writing serialize for the Record to get byte arrayy
// My thought is this func can't error cause it's appending in mem
// to a buf values that are already in the types they should be so no err returned
func (r Record) serialize() []byte {
	buf := make([]byte, 0, 4+8+4+4+r.header.keyLen+r.header.valLen)

	// header
	// pad 4 bytes to add CRC later
	buf = binary.LittleEndian.AppendUint32(buf, r.header.crc)
	buf = binary.LittleEndian.AppendUint64(buf, r.header.ts)
	buf = binary.LittleEndian.AppendUint32(buf, r.header.keyLen)
	buf = binary.LittleEndian.AppendUint32(buf, r.header.valLen)

	//record values
	buf = append(buf, r.key...)

	buf = append(buf, r.val...)

	return buf

}

// again not returning error as I don't see how reading from a
// slice and assigning to a diff part of memory can fail
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

// Put appends a given Record to a given file

// Decision: Should the caller to this package be giving
// the file or should we be choosing to put it in a file
// to me just us putting it in a file makes sense, so we need a global
// file and a descriptor to it should be possible to created
// should we expect file to already be there when this binary is executing ?
// A postgres CREATE DATABASE; on a database cluster then creates files under neath right
// so starting a file should be our job, only the caller needs to give us key and values and we use a global file as the store.

// So what I am expressing is the moment somebody boots a binary with they should get a file created I am rethinking this should be main's job right
// this package is only he package for Record based ops on a given file, I am going to expect a descriptor we'll sort in main what descriptor to send
func Put(key []byte, val []byte, fd os.File) error {

	var r Record
	// When should we calc CRC once we have rest all the pieces of the record ?
	// I think yes because let's say a byte on the timestamp field went corrupt then we should still know right
	// or let's say the len value is not matching the actual length of the records but what's the point if I have fixed the length to 512 bytes shit that's waste need to fix this
	// how do I express that key and value is just array of bytes whose max len can be 512 but don't allocate 512 every time. Should I just use slice then the memory allocation is runtime's problem cool then.
	// so first let's check if the passed on byte slices are greater than the len we expect to store although I am already worried that can external caller can load unbounded amount of memory into this program
	// that's already bad let me see that later

	// validate everything before using
	if len(key) > MAXLENKEYVAL {
		return fmt.Errorf("Key too long")
	}

	if len(val) > MAXLENKEYVAL {
		return fmt.Errorf("Val too long")
	}

	if _, err := fd.Stat(); err != nil {
		return fmt.Errorf("Invalid file descriptor can not append")
	}

	// Creating the record
	// current nanosecond ts this is the rep of when the record is being stored though since it's upto ns level it is off cause we do some ops after this before we store.
	// is that acceptable (unsure?)
	// I am sure this conversion never fails cause then len of key never goes beyond 512
	r.header.keyLen = uint32(len(key))
	r.header.valLen = uint32(len(val))
	r.header.ts = uint64(time.Now().UnixNano())
	r.key = key
	r.val = val

	// calc LSB CRC
	// Now I am stuck  I have the checksum as part of my record so now I am unable to calc
	// the checksum before the whole record is formed so I fucked up at the real start
	data := r.serialize()
	r.header.crc = crc32.Checksum(data[4:], crc32.IEEETable)

	crcBuf := make([]byte, 0, 4)
	crcBuf = binary.LittleEndian.AppendUint32(crcBuf, r.header.crc)
	data = append(crcBuf, data[4:]...)

	// append to a file
	_, err := fd.Write(data)

	if err != nil {
		return fmt.Errorf("Failed to write to file: %s", fd.Name())
	}
	err = fd.Sync()
	if err != nil {
		return fmt.Errorf("Failed to sync file to storage: %s", fd.Name())
	}

	return nil
}
