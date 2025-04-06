package models

type SecretRef struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Name      string `json:"name" yaml:"name"`
}

type Certificate struct {
	Name        string            `json:"name" yaml:"name"`
	Domains     []string          `json:"domains" yaml:"domains"`
	DNSProvider string            `json:"dns" yaml:"dns"`
	Server      string            `json:"server" yaml:"server"`
	Email       string            `json:"email" yaml:"email"`
	Secrets     []SecretRef       `json:"secrets" yaml:"secrets"`
	Envs        map[string]string `json:"envs,omitempty" yaml:"envs,omitempty"`
	IssuedAt    string            `json:"issued_at,omitempty" yaml:"issued_at,omitempty"`
	ExpiresAt   string            `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
	CertData    string            `json:"cert_data,omitempty" yaml:"cert_data,omitempty"` // Base64 encoded certificate data
	KeyData     string            `json:"key_data,omitempty" yaml:"key_data,omitempty"`   // Base64 encoded key data
}

// CertificateContext 用于持久化存储证书信息
type CertificateContext struct {
	Certificates map[string]Certificate `json:"certificates" yaml:"certificates"`
}
