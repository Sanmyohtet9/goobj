package goobj

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
)

const supportedGoObjVersion = 1

var magicHeader = []byte("\x00\x00go19ld")
var magicFooter = []byte("\xffgo19ld")

// File represents a go object file.
type File struct {
	symbols          []Symbol
	symbolReferences []SymbolReference
}

// SymbolReference represents a symbol's name and its version.
type SymbolReference struct {
	Name    string
	Version int64
}

// Symbol describes metadata associated with data block.
type Symbol struct {
	IDIndex     int64
	Kind        SymKind
	Size        int64
	DupOK       bool
	Local       bool
	Typelink    bool
	GoTypeIndex int64
	Data        DataAddr
	Relocations []Relocation
	// STEXT type has additional fields
	stextFields *StextFields
}

// SymKind represents a type of symbol
type SymKind uint8

// taken from go1.10 cmd/internal/objabi
const (
	// An otherwise invalid zero value for the type
	Sxxx SymKind = iota
	// Executable instructions
	STEXT
	// Read only static data
	SRODATA
	// Static data that does not contain any pointers
	SNOPTRDATA
	// Static data
	SDATA
	// Statically data that is initially all 0s
	SBSS
	// Statically data that is initially all 0s and does not contain pointers
	SNOPTRBSS
	// Thread-local data that is initially all 0s
	STLSBSS
	// Debugging data
	SDWARFINFO
	SDWARFRANGE
	SDWARFLOC
)

func (kind SymKind) String() string {
	switch kind {
	case Sxxx:
		return "INVALID"
	case STEXT:
		return "STEXT"
	case SRODATA:
		return "SRODATA"
	case SNOPTRDATA:
		return "SNOPTRDATA"
	case SDATA:
		return "SDATA"
	case SBSS:
		return "SBSS"
	case SNOPTRBSS:
		return "SNOPTRBSS"
	case STLSBSS:
		return "STLSBSS"
	case SDWARFINFO:
		return "SDWARFINFO"
	case SDWARFRANGE:
		return "SDWARFRANGE"
	case SDWARFLOC:
		return "SDWARFLOC"
	default:
		return "UNKNOWN"
	}
}

// Relocation represents a symbol to be relocated and how to relocate it.
type Relocation struct {
	Offset  int64
	Size    int64
	Type    RelocType
	Add     int64
	IDIndex int64
}

// RelocType describes a way to relocate a symbol.
type RelocType int32

// taken from go1.10 cmd/internal/objabi
const (
	R_ADDR RelocType = 1 + iota
	// R_ADDRPOWER relocates a pair of "D-form" instructions (instructions with 16-bit
	// immediates in the low half of the instruction word), usually addis followed by
	// another add or a load, inserting the "high adjusted" 16 bits of the address of
	// the referenced symbol into the immediate field of the first instruction and the
	// low 16 bits into that of the second instruction.
	R_ADDRPOWER
	// R_ADDRARM64 relocates an adrp, add pair to compute the address of the
	// referenced symbol.
	R_ADDRARM64
	// R_ADDRMIPS (only used on mips/mips64) resolves to the low 16 bits of an external
	// address, by encoding it into the instruction.
	R_ADDRMIPS
	// R_ADDROFF resolves to a 32-bit offset from the beginning of the section
	// holding the data being relocated to the referenced symbol.
	R_ADDROFF // 5
	// R_WEAKADDROFF resolves just like R_ADDROFF but is a weak relocation.
	// A weak relocation does not make the symbol it refers to reachable,
	// and is only honored by the linker if the symbol is in some other way
	// reachable.
	R_WEAKADDROFF
	R_SIZE
	R_CALL // 8
	R_CALLARM
	R_CALLARM64
	R_CALLIND // 11
	R_CALLPOWER
	// R_CALLMIPS (only used on mips64) resolves to non-PC-relative target address
	// of a CALL (JAL) instruction, by encoding the address into the instruction.
	R_CALLMIPS
	R_CONST
	R_PCREL // 15
	// R_TLS_LE, used on 386, amd64, and ARM, resolves to the offset of the
	// thread-local symbol from the thread local base and is used to implement the
	// "local exec" model for tls access (r.Sym is not set on intel platforms but is
	// set to a TLS symbol -- runtime.tlsg -- in the linker when externally linking).
	R_TLS_LE
	// R_TLS_IE, used 386, amd64, and ARM resolves to the PC-relative offset to a GOT
	// slot containing the offset from the thread-local symbol from the thread local
	// base and is used to implemented the "initial exec" model for tls access (r.Sym
	// is not set on intel platforms but is set to a TLS symbol -- runtime.tlsg -- in
	// the linker when externally linking).
	R_TLS_IE
	R_GOTOFF
	R_PLT0
	R_PLT1
	R_PLT2
	R_USEFIELD
	// R_USETYPE resolves to an *rtype, but no relocation is created. The
	// linker uses this as a signal that the pointed-to type information
	// should be linked into the final binary, even if there are no other
	// direct references. (This is used for types reachable by reflection.)
	R_USETYPE
	// R_METHODOFF resolves to a 32-bit offset from the beginning of the section
	// holding the data being relocated to the referenced symbol.
	// It is a variant of R_ADDROFF used when linking from the uncommonType of a
	// *rtype, and may be set to zero by the linker if it determines the method
	// text is unreachable by the linked program.
	R_METHODOFF // 24
	R_POWER_TOC
	R_GOTPCREL
	// R_JMPMIPS (only used on mips64) resolves to non-PC-relative target address
	// of a JMP instruction, by encoding the address into the instruction.
	// The stack nosplit check ignores this since it is not a function call.
	R_JMPMIPS

	// R_DWARFSECREF resolves to the offset of the symbol from its section.
	// Target of relocation must be size 4 (in current implementation).
	R_DWARFSECREF // 28

	// R_DWARFFILEREF resolves to an index into the DWARF .debug_line
	// file table for the specified file symbol. Must be applied to an
	// attribute of form DW_FORM_data4.
	R_DWARFFILEREF // 29

	// Platform dependent relocations. Architectures with fixed width instructions
	// have the inherent issue that a 32-bit (or 64-bit!) displacement cannot be
	// stuffed into a 32-bit instruction, so an address needs to be spread across
	// several instructions, and in turn this requires a sequence of relocations, each
	// updating a part of an instruction. This leads to relocation codes that are
	// inherently processor specific.

	// Arm64.

	// Set a MOV[NZ] immediate field to bits [15:0] of the offset from the thread
	// local base to the thread local variable defined by the referenced (thread
	// local) symbol. Error if the offset does not fit into 16 bits.
	R_ARM64_TLS_LE

	// Relocates an ADRP; LD64 instruction sequence to load the offset between
	// the thread local base and the thread local variable defined by the
	// referenced (thread local) symbol from the GOT.
	R_ARM64_TLS_IE

	// R_ARM64_GOTPCREL relocates an adrp, ld64 pair to compute the address of the GOT
	// slot of the referenced symbol.
	R_ARM64_GOTPCREL

	// PPC64.

	// R_POWER_TLS_LE is used to implement the "local exec" model for tls
	// access. It resolves to the offset of the thread-local symbol from the
	// thread pointer (R13) and inserts this value into the low 16 bits of an
	// instruction word.
	R_POWER_TLS_LE

	// R_POWER_TLS_IE is used to implement the "initial exec" model for tls access. It
	// relocates a D-form, DS-form instruction sequence like R_ADDRPOWER_DS. It
	// inserts to the offset of GOT slot for the thread-local symbol from the TOC (the
	// GOT slot is filled by the dynamic linker with the offset of the thread-local
	// symbol from the thread pointer (R13)).
	R_POWER_TLS_IE

	// R_POWER_TLS marks an X-form instruction such as "MOVD 0(R13)(R31*1), g" as
	// accessing a particular thread-local symbol. It does not affect code generation
	// but is used by the system linker when relaxing "initial exec" model code to
	// "local exec" model code.
	R_POWER_TLS

	// R_ADDRPOWER_DS is similar to R_ADDRPOWER above, but assumes the second
	// instruction is a "DS-form" instruction, which has an immediate field occupying
	// bits [15:2] of the instruction word. Bits [15:2] of the address of the
	// relocated symbol are inserted into this field; it is an error if the last two
	// bits of the address are not 0.
	R_ADDRPOWER_DS

	// R_ADDRPOWER_PCREL relocates a D-form, DS-form instruction sequence like
	// R_ADDRPOWER_DS but inserts the offset of the GOT slot for the referenced symbol
	// from the TOC rather than the symbol's address.
	R_ADDRPOWER_GOT

	// R_ADDRPOWER_PCREL relocates two D-form instructions like R_ADDRPOWER, but
	// inserts the displacement from the place being relocated to the address of the
	// the relocated symbol instead of just its address.
	R_ADDRPOWER_PCREL

	// R_ADDRPOWER_TOCREL relocates two D-form instructions like R_ADDRPOWER, but
	// inserts the offset from the TOC to the address of the relocated symbol
	// rather than the symbol's address.
	R_ADDRPOWER_TOCREL

	// R_ADDRPOWER_TOCREL relocates a D-form, DS-form instruction sequence like
	// R_ADDRPOWER_DS but inserts the offset from the TOC to the address of the the
	// relocated symbol rather than the symbol's address.
	R_ADDRPOWER_TOCREL_DS

	// R_PCRELDBL relocates s390x 2-byte aligned PC-relative addresses.
	// TODO(mundaym): remove once variants can be serialized - see issue 14218.
	R_PCRELDBL

	// R_ADDRMIPSU (only used on mips/mips64) resolves to the sign-adjusted "upper" 16
	// bits (bit 16-31) of an external address, by encoding it into the instruction.
	R_ADDRMIPSU
	// R_ADDRMIPSTLS (only used on mips64) resolves to the low 16 bits of a TLS
	// address (offset from thread pointer), by encoding it into the instruction.
	R_ADDRMIPSTLS
	// R_ADDRCUOFF resolves to a pointer-sized offset from the start of the
	// symbol's DWARF compile unit.
	R_ADDRCUOFF // 44
)

func (relocType RelocType) String() string {
	switch relocType {
	case R_ADDR:
		return "R_ADDR"
	case R_ADDRPOWER:
		return "R_ADDRPOWER"
	case R_ADDRARM64:
		return "R_ADDRARM64"
	case R_ADDRMIPS:
		return "R_ADDRMIPS"
	case R_ADDROFF:
		return "R_ADDROFF"
	case R_WEAKADDROFF:
		return "R_WEAKADDROFF"
	case R_SIZE:
		return "R_SIZE"
	case R_CALL:
		return "R_CALL"
	case R_CALLARM:
		return "R_CALLARM"
	case R_CALLARM64:
		return "R_CALLARM64"
	case R_CALLIND:
		return "R_CALLIND"
	case R_CALLPOWER:
		return "R_CALLPOWER"
	case R_CALLMIPS:
		return "R_CALLMIPS"
	case R_CONST:
		return "R_CONST"
	case R_PCREL:
		return "R_PCREL"
	case R_TLS_LE:
		return "R_TLS_LE"
	case R_TLS_IE:
		return "R_TLS_IE"
	case R_GOTOFF:
		return "R_GOTOFF"
	case R_PLT0:
		return "R_PLT0"
	case R_PLT1:
		return "R_PLT1"
	case R_PLT2:
		return "R_PLT2"
	case R_USEFIELD:
		return "R_USEFIELD"
	case R_USETYPE:
		return "R_USETYPE"
	case R_METHODOFF:
		return "R_METHODOFF"
	case R_POWER_TOC:
		return "R_POWER_TOC"
	case R_GOTPCREL:
		return "R_GOTPCREL"
	case R_JMPMIPS:
		return "R_JMPMIPS"
	case R_DWARFSECREF:
		return "R_DWARFSECREF"
	case R_DWARFFILEREF:
		return "R_DWARFFILEREF"
	case R_ARM64_TLS_LE:
		return "R_ARM64_TLS_LE"
	case R_ARM64_TLS_IE:
		return "R_ARM64_TLS_IE"
	case R_ARM64_GOTPCREL:
		return "R_ARM64_GOTPCREL"
	case R_POWER_TLS_LE:
		return "R_POWER_TLS_LE"
	case R_POWER_TLS_IE:
		return "R_POWER_TLS_IE"
	case R_POWER_TLS:
		return "R_POWER_TLS"
	case R_ADDRPOWER_DS:
		return "R_ADDRPOWER_DS"
	case R_ADDRPOWER_GOT:
		return "R_ADDRPOWER_GOT"
	case R_ADDRPOWER_PCREL:
		return "R_ADDRPOWER_PCREL"
	case R_ADDRPOWER_TOCREL:
		return "R_ADDRPOWER_TOCREL"
	case R_ADDRPOWER_TOCREL_DS:
		return "R_ADDRPOWER_TOCREL_DS"
	case R_PCRELDBL:
		return "R_PCRELDBL"
	case R_ADDRMIPSU:
		return "R_ADDRMIPSU"
	case R_ADDRMIPSTLS:
		return "R_ADDRMIPSTLS"
	case R_ADDRCUOFF:
		return "R_ADDRCUOFF"
	default:
		return "Unknown"
	}
}

// StextFields represents additional metadata STEXT-type symbol have.
type StextFields struct {
	Args       int64
	Frame      int64
	Leaf       bool
	CFunc      bool
	TypeMethod bool
	SharedFunc bool
	NoSplit    bool
	Local      []Local
	// pcln table
	PCSP           DataAddr
	PCFile         DataAddr
	PCLine         DataAddr
	PCInline       DataAddr
	PCData         []DataAddr
	FuncDataIndex  []int64
	FuncDataOffset []int64
	FileIndex      []int64
}

// Local represents a local variable including input args and output.
type Local struct {
	AsymIndex   int64
	Offset      int64
	Type        int64
	GotypeIndex int64
}

// DataAddr represents a location of data block.
type DataAddr struct {
	Size, Offset int64
}

// Parse parses a given go object file
func Parse(f *os.File) (*File, error) {
	return nil, nil
}

type parser struct {
	reader       readerWithCounter
	currDataAddr int64
	Err          error
	File
}

func newParser(raw *bufio.Reader) *parser {
	return &parser{reader: readerWithCounter{raw: raw}}
}

func (p *parser) skipHeader() {
	if p.Err != nil {
		return
	}

	buff := make([]byte, len(magicHeader))
	_, err := p.reader.read(buff)
	if err != nil {
		p.Err = err
		return
	}

	for !reflect.DeepEqual(buff, magicHeader) {
		b, err := p.reader.readByte()
		if err != nil {
			p.Err = err
			return
		}

		buff = append(buff[1:], b)
	}
}

func (p *parser) checkVersion() {
	if p.Err != nil {
		return
	}

	version, err := p.reader.readByte()
	if err != nil {
		p.Err = err
		return
	}

	if version != 1 {
		p.Err = fmt.Errorf("unexpected version: %d", version)
	}
}

func (p *parser) skipDependencies() {
	if p.Err != nil {
		return
	}

	for {
		b, err := p.reader.readByte()
		if err != nil {
			p.Err = err
			return
		}

		if b == 0 {
			return
		}
	}
}

func (p *parser) readReferences() {
	if p.Err != nil {
		return
	}

	for {
		b, err := p.reader.readByte()
		if err != nil {
			p.Err = err
			return
		}

		if b == 0xff {
			return
		} else if b != 0xfe {
			p.Err = fmt.Errorf("sanity check failed: %#x ", b)
			return
		}

		p.readReference()
		if p.Err != nil {
			return
		}
	}
}

func (p *parser) readReference() {
	symbolName, err := p.reader.readString()
	if err != nil {
		p.Err = err
		return
	}

	symbolVersion, err := p.reader.readVarint()
	if err != nil {
		p.Err = err
		return
	}

	p.symbolReferences = append(p.symbolReferences, SymbolReference{symbolName, symbolVersion})
}

type readerWithCounter struct {
	raw          *bufio.Reader
	numReadBytes int64
}

func (r *readerWithCounter) readVarint() (int64, error) {
	var value uint64
	var shift uint64
	for {
		b, err := r.readByte()
		if err != nil {
			return 0, err
		}

		value += uint64(b&0x7f) << shift
		if (b>>7)&0x1 == 0 {
			break
		}
		shift += 7
	}
	return zigzagDecode(value), nil
}

func (r *readerWithCounter) readString() (string, error) {
	len, err := r.readVarint()
	if err != nil {
		return "", err
	}

	buff := make([]byte, len)
	numRead := 0
	for numRead != int(len) {
		n, err := r.read(buff[numRead:])
		if err != nil {
			return "", err
		}
		numRead += n
	}

	return string(buff), nil
}

func (r *readerWithCounter) readByte() (b byte, err error) {
	b, err = r.raw.ReadByte()
	r.numReadBytes++
	return
}

func (r *readerWithCounter) read(p []byte) (n int, err error) {
	n, err = r.raw.Read(p)
	r.numReadBytes += int64(n)
	return
}