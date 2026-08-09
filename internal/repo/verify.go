package repo

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
)

// rokfossKey는 ROKFOSS 저장소 서명 키입니다.
// 바이너리에 박아 두어 첫 실행부터 서명을 검증할 수 있게 합니다.
//
//go:embed keys/rokfoss.asc
var rokfossKey []byte

// PinnedFingerprint는 신뢰하는 주 키의 지문입니다.
// 저장소가 다른 키로 서명하기 시작하면 검증이 실패하도록 못 박아 둡니다.
const PinnedFingerprint = "FA9794DD072735B53A6F336B31FDB273B4C40C47"

// SignerInfo는 서명 검증에 성공한 키의 정보입니다.
type SignerInfo struct {
	Fingerprint string
	Identity    string
}

// verifyInRelease는 InRelease(클리어사인 문서)의 서명을 확인하고
// 서명된 본문을 돌려줍니다.
func verifyInRelease(data []byte) ([]byte, *SignerInfo, error) {
	block, _ := clearsign.Decode(data)
	if block == nil {
		return nil, nil, fmt.Errorf("InRelease가 서명된 문서가 아닙니다")
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(rokfossKey))
	if err != nil {
		return nil, nil, fmt.Errorf("내장된 저장소 키를 읽지 못했습니다: %w", err)
	}

	signer, err := openpgp.CheckDetachedSignature(
		keyring,
		bytes.NewReader(block.Bytes),
		block.ArmoredSignature.Body,
		nil,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("저장소 서명 검증에 실패했습니다: %w", err)
	}

	fp := strings.ToUpper(hex.EncodeToString(signer.PrimaryKey.Fingerprint))
	if fp != PinnedFingerprint {
		return nil, nil, fmt.Errorf("서명 키가 신뢰 목록과 다릅니다 (%s)", fp)
	}

	info := &SignerInfo{Fingerprint: fp}
	for name := range signer.Identities {
		info.Identity = name
		break
	}
	return block.Plaintext, info, nil
}

// verifySHA256은 내용의 해시가 기대값과 같은지 확인합니다.
func verifySHA256(data []byte, want string) error {
	if want == "" {
		return fmt.Errorf("기대 해시값이 비어 있습니다")
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("SHA256 불일치 (기대 %s, 실제 %s)", want, got)
	}
	return nil
}

// sha256Reader는 스트림을 읽으면서 해시를 계산합니다.
func sha256Reader(r io.Reader) (io.Reader, func() string) {
	h := sha256.New()
	return io.TeeReader(r, h), func() string { return hex.EncodeToString(h.Sum(nil)) }
}
