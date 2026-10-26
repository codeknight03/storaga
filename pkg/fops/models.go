package fops

// On-disk record layout (little-endian, 20-byte header):
//
//	| crc u32 | ts u64 | keyLen u32 | valLen u32 | key bytes … | val bytes … |
//	  0         4        12           16           20
//
// crc covers every byte after itself (header + key + val).

// header is the fixed-size prefix of every record.
type header struct {
	// CRC-32 (IEEE) of everything after this field.
	crc uint32

	// Unix time in nanoseconds when the record was built.
	// Open: how many bytes a timestamp needs per record — settled on u64 Unix ns,
	// since that's the common way to store it.
	ts uint64

	// Decision: width of the key/value length fields.
	// Options:
	//   - uint64: 2^64 bytes (~16 EiB), far beyond any real need.
	//   - uint32 + a max size: Redis caps keys/values at 512 MB (2^29 bytes).
	//   (Postgres also limits columns, so DBs do impose limits.)
	// Why: uint32 with a 512 MiB cap (MAXLENKEYVAL) matches a real system and
	//      fits easily in 32 bits.
	// Revisit if: TBD.
	keyLen uint32
	valLen uint32
}

// Record is the unit every append (and later, read) operates on.
type Record struct {
	header header
	key    []byte
	val    []byte
}
