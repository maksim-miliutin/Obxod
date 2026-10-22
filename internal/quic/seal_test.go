package quic

import (
	"encoding/hex"
	"testing"
)

// The CRYPTO frame and ClientHello of RFC 9001 Appendix A.2, before the PADDING.
const goldFrames = "060040f1010000ed0303ebf8fa56f12939b9584a3896472ec40bb863cfd3e86804fe3a47f06a2b69484c00000413011302010000c000000010000e00000b6578616d706c652e636f6dff01000100000a00080006001d0017001800100007000504616c706e000500050100000000003300260024001d00209370b2c9caa47fbabaf4559fedba753de171fa71f50f1ce15d43e994ec74d748002b0003020304000d0010000e0403050306030203080408050806002d00020101001c00024001003900320408ffffffffffffffff05048000ffff07048000ffff0801100104800075300901100f088394c8f03e51570806048000ffff"

// RFC 9001 Appendix A.2: sealing the sample client Initial must reproduce the
// protected bytes the RFC prints. Matching the prefix pins the nonce, the aad and
// the header-protection mask together, since every one of them feeds these bytes.
func TestSealReproducesRFC9001Initial(t *testing.T) {
	header := unhex(t, "c300000001088394c8f03e5157080000449e00000002")
	frames := unhex(t, goldFrames)

	if len(frames) != 245 {
		t.Fatalf("gold frames are %d bytes, want 245", len(frames))
	}

	plaintext := append(frames, make([]byte, 1162-len(frames))...)
	packet := append(append([]byte{}, header...), plaintext...)

	keys, err := InitialKeys(unhex(t, "8394c8f03e515708"))
	if err != nil {
		t.Fatal(err)
	}

	out, err := keys.Seal(packet, 18, 4)
	if err != nil {
		t.Fatal(err)
	}

	if len(out) != 1200 {
		t.Fatalf("sealed %d bytes, want the 1200 of a client Initial", len(out))
	}

	const prefix = "c000000001088394c8f03e5157080000449e7b9aec34d1b1c98dd7689fb8ec11d2"
	if got := hex.EncodeToString(out[:len(prefix)/2]); got != prefix {
		t.Errorf("protected prefix\n got %s\nwant %s", got, prefix)
	}
}
