package client

type RegistryStatus struct {
	Status string `json:"status"`
}

type QuayConfig struct {
	Config map[string]interface{} `json:"config"`
}

// type QuayKeys struct {
// 	Keys []interface{} `json:"keys"`
// }

type QuayStatusResponse struct {
	Status bool   `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type SecurityScannerKey struct {
	Kid        string `json:"kid"`
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	Service    string `json:"service"`
}

type SetupDatabaseResponse struct {
	Logs []LogMessage `json:"logs"`
}

type LogMessage struct {
	Message string `json:"message"`
	Level   string `json:"level"`
}

type QuayCreateSuperuserRequest struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmpassword"`
	Email           string `json:"email"`
}

type QuayCreateSecurityScannerKeyRequest struct {
	Name       string      `json:"name"`
	Service    string      `json:"service"`
	Expiration interface{} `json:"expiration"`
	Notes      string      `json:"notes"`
}

type ConfigFileStatus struct {
	Exists bool `json:"exists"`
}

type StringValue struct {
	Value string
}
