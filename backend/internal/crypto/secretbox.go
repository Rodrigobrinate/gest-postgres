// Package crypto cifra segredos (senhas de servidores gerenciados, tokens de storage)
// antes de gravar no banco de metadados. AES-256-GCM com chave fixa vinda de config.
// Chave rotacionada = re-cifrar tudo; fora de escopo do MVP.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

type SecretBox struct {
	gcm cipher.AEAD
}

func NewSecretBox(hexKey string) (*SecretBox, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("chave de criptografia inválida: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("criando cipher AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("criando GCM: %w", err)
	}
	return &SecretBox{gcm: gcm}, nil
}

// Seal cifra plaintext e retorna nonce+ciphertext codificado em hex.
func (s *SecretBox) Seal(plaintext string) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("gerando nonce: %w", err)
	}
	ciphertext := s.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Open decifra o valor produzido por Seal.
func (s *SecretBox) Open(sealed string) (string, error) {
	data, err := hex.DecodeString(sealed)
	if err != nil {
		return "", fmt.Errorf("decodificando hex: %w", err)
	}
	nonceSize := s.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext muito curto")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := s.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decifrando: %w", err)
	}
	return string(plaintext), nil
}
