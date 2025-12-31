package sshutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	"golang.org/x/crypto/ssh"
)

// KeyPair holds a generated SSH key pair
type KeyPair struct {
	PublicKey  string // OpenSSH format (ssh-ed25519 AAAA... comment)
	PrivateKey string // PEM format
}

// GenerateKeyPair creates a new Ed25519 SSH key pair
func GenerateKeyPair(comment string) (*KeyPair, error) {
	// Generate Ed25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Convert public key to SSH format
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Format public key as OpenSSH authorized_keys format
	pubKeyStr := string(ssh.MarshalAuthorizedKey(sshPubKey))
	if comment != "" {
		// ssh.MarshalAuthorizedKey adds a newline, insert comment before it
		pubKeyStr = pubKeyStr[:len(pubKeyStr)-1] + " " + comment + "\n"
	}

	// Encode private key as PEM
	privKeyPEM, err := ssh.MarshalPrivateKey(privKey, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	privKeyStr := string(pem.EncodeToMemory(privKeyPEM))

	return &KeyPair{
		PublicKey:  pubKeyStr,
		PrivateKey: privKeyStr,
	}, nil
}

// FormatSSHCommand returns the SSH command to connect to a VM
func FormatSSHCommand(user, host, privateKeyPath string) string {
	return fmt.Sprintf("ssh -i %s %s@%s", privateKeyPath, user, host)
}

// FormatGCloudSSHCommand returns the gcloud SSH command
func FormatGCloudSSHCommand(vmName, zone, project string) string {
	return fmt.Sprintf("gcloud compute ssh %s --zone=%s --project=%s", vmName, zone, project)
}
