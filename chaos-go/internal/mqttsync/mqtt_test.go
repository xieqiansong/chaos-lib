package mqttsync

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// 固定测试密钥（hex 64 字符 → 32 字节）。仅用于单元测试，非真实密钥。
const testKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// withKey 临时把包级加密状态置为给定密钥后运行 fn，结束后复位，避免影响其它测试。
func withKey(t *testing.T, key []byte) {
	t.Helper()
	encryptMu.Lock()
	encryptKey = key
	effectiveEncrypt = true
	encryptMu.Unlock()
	t.Cleanup(func() {
		encryptMu.Lock()
		encryptKey = nil
		effectiveEncrypt = false
		encryptMu.Unlock()
	})
}

func TestLoadKeyAcceptsHexAndBase64(t *testing.T) {
	if k := loadKey(testKeyHex); len(k) != 32 {
		t.Fatalf("hex 解析失败，长度=%d", len(k))
	}
	raw, _ := hex.DecodeString(testKeyHex)
	b64 := base64.StdEncoding.EncodeToString(raw)
	if k := loadKey(b64); len(k) != 32 {
		t.Fatalf("base64 解析失败，长度=%d", len(k))
	}
	if k := loadKey("tooshort"); k != nil {
		t.Fatalf("无效密钥应返回 nil，得到长度=%d", len(k))
	}
}

func TestSealOpenRoundtrip(t *testing.T) {
	withKey(t, loadKey(testKeyHex))

	inner := []byte(`{"id":"abc","node_id":"n1","channel":"broadcast","payload":"hello","ts":"2026-01-01T00:00:00Z"}`)
	env, err := seal(inner)
	if err != nil {
		t.Fatalf("seal 失败: %v", err)
	}
	// 信封应可被 JSON 解析且 enc 字段正确
	var e envelope
	if err := json.Unmarshal(env, &e); err != nil {
		t.Fatalf("信封不是合法 JSON: %v", err)
	}
	if e.Enc != "aes-256-gcm" || e.Data == "" {
		t.Fatalf("信封字段异常: %+v", e)
	}
	got, err := open(env)
	if err != nil {
		t.Fatalf("open 失败: %v", err)
	}
	if string(got) != string(inner) {
		t.Fatalf("往返不一致: got=%s want=%s", got, inner)
	}
}

func TestOpenRejectsTampered(t *testing.T) {
	withKey(t, loadKey(testKeyHex))

	inner := []byte(`{"id":"x","node_id":"n1","channel":"b","payload":"secret","ts":"t"}`)
	env, _ := seal(inner)
	var e envelope
	if err := json.Unmarshal(env, &e); err != nil {
		t.Fatalf("解析信封失败: %v", err)
	}
	// 篡改 base64 数据中的一个字符（仍可能是合法 base64，但 GCM tag 必失败）
	b := []byte(e.Data)
	b[len(b)-1] ^= 0x01
	e.Data = string(b)
	corrupted, _ := json.Marshal(e)
	if _, err := open(corrupted); err == nil {
		t.Fatalf("篡改后的密文应当解密失败，却成功了")
	}
}

func TestOpenWrongKeyFails(t *testing.T) {
	withKey(t, loadKey(testKeyHex))

	inner := []byte(`{"id":"x","node_id":"n1","channel":"b","payload":"secret","ts":"t"}`)
	env, _ := seal(inner)

	// 换成错误密钥再尝试解密
	wrong := "ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100"
	encryptMu.Lock()
	encryptKey = loadKey(wrong)
	encryptMu.Unlock()
	if _, err := open(env); err == nil {
		t.Fatalf("使用错误密钥应解密失败")
	}
}

func TestOpenPlaintextLegacy(t *testing.T) {
	// 模拟旧版本明文报文（无 enc 字段），open 应原样返回以兼容滚动升级
	plain := []byte(`{"id":"y","node_id":"n2","channel":"broadcast","payload":"hi","ts":"t"}`)
	got, err := open(plain)
	if err != nil {
		t.Fatalf("明文旧消息应被兼容，但报错: %v", err)
	}
	if string(got) != string(plain) {
		t.Fatalf("明文旧消息应原样返回: got=%s", got)
	}
}

func TestEffectiveEncryptReflectsState(t *testing.T) {
	encryptMu.Lock()
	effectiveEncrypt = false
	encryptMu.Unlock()
	if EffectiveEncrypt() {
		t.Fatalf("未启用时 EffectiveEncrypt 应返回 false")
	}
	withKey(t, loadKey(testKeyHex))
	if !EffectiveEncrypt() {
		t.Fatalf("启用后 EffectiveEncrypt 应返回 true")
	}
}
