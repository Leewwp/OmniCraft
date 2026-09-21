package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// 测试密钥运行时构造（strings.Repeat），文件内不出现任何高熵密钥字面量
// ——gitleaks generic-api-key 对 SECRET 关键字旁的字面量会拦截（SP-25 FR-08
// 首轮实踩）；本文件不走 config_test.go 的文件级豁免通道。
func testEncryptionSecret() string { return strings.Repeat("ab", 32) }

// SP-25 中-4：密钥链双缺失时拒绝派生（不得退化 sha256("") 公开常量）；
// 有密钥时 roundtrip；存量无前缀明文透传保持兼容。

func TestEncryptDecryptLLMAPIKeyRoundtrip(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", testEncryptionSecret())
	enc, err := encryptLLMAPIKey("sk-provider-key-abcdef")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(enc, "v1:"), "encrypted value must carry the v1: prefix")

	dec, err := decryptLLMAPIKey(enc)
	require.NoError(t, err)
	require.Equal(t, "sk-provider-key-abcdef", dec)
}

func TestLLMEncryptionKeyFailsClosedWithoutSecret(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "")
	t.Setenv("JWT_SECRET", "")

	_, err := llmEncryptionKey()
	require.ErrorIs(t, err, ErrLLMKeyEncryptionSecretMissing)

	// 写路径拒绝新密钥，而不是伪加密落库。
	_, err = encryptLLMAPIKey("sk-any")
	require.ErrorIs(t, err, ErrLLMKeyEncryptionSecretMissing)
}

func TestLLMEncryptionKeyJWTSecretFallback(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "")
	t.Setenv("JWT_SECRET", testEncryptionSecret())

	enc, err := encryptLLMAPIKey("sk-via-jwt-fallback")
	require.NoError(t, err)
	dec, err := decryptLLMAPIKey(enc)
	require.NoError(t, err)
	require.Equal(t, "sk-via-jwt-fallback", dec)
}

func TestDecryptLLMAPIKeyLegacyPlaintextPassesThrough(t *testing.T) {
	t.Setenv("LLM_KEY_ENCRYPTION_SECRET", "")
	t.Setenv("JWT_SECRET", "")

	// 无 v1: 前缀的存量明文：透传兼容（不因密钥缺失而拒绝读取）。
	dec, err := decryptLLMAPIKey("legacy-plaintext-key")
	require.NoError(t, err)
	require.Equal(t, "legacy-plaintext-key", dec)

	// v1: 密文在密钥缺失时显式报错（fail-fast，非静默降级）。
	_, err = decryptLLMAPIKey("v1:AAAA")
	require.ErrorIs(t, err, ErrLLMKeyEncryptionSecretMissing)
}
