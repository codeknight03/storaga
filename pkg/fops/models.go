package fops

// Defines the appendable records and parts of them

// Final record we are looking to store on disk
// | CRC | timestamp | key len | value len | key bytes … | value bytes … |

// header is the representative type for the CRC timestamp and the two lens
type header struct {

	// implementing CRC-32 later so going for a uint32 ( to save the 32 bits (4 bytes ) / reccord as this is 64 bit system)
	crc uint32
	// for timestamp using the well recognized global timestamp till nanosecond precision
	// ( unsure of how many bytes is this per record and how do I even calculate that)
	// later found that really mostly this is stored as he unix timestamp only so
	ts uint64

	// Deciding the size of the length of a key and value before we know how long can they be, should we have a limit on how long a record can be do databases do that ?
	// ( refering to DBs because they are the standard of file IO) , I think they do for example postgres limits num column
	// Using a full 64bit integer here as lens can't be floating I think this should suffice a 64bit len means I am
	// storing something 2^64 byte long, i.e. something beyond this universe can store
	// and since I am stupid I used this to calculate that which is ridiculous >>> (256*256*256*256*256*256*256*256)/1024/1024/1024/1024
	//16777216
	// that's insane I think let's see what redis allows they allow 512mb / key or value
	// which can be reped by a 2^29 length
	keyLen uint32
	valLen uint32
}

// Record reps a full record this the unit of operation all
// appends and gets will be of this quantum
type Record struct {
	header header
	key    []byte
	val    []byte
}
