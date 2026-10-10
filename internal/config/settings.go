package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
)

// CredentialsSetting is the guarded key of the proxy credentials, which read host secrets and send them out.
const CredentialsSetting = "run.proxy.credentials"

// GuardedSetting is a key of a config file that reaches past the container, so changing it needs the user's approval.
type GuardedSetting struct {
	Key   string
	Value any
}

// IsSet reports whether the setting has a value; an empty list counts as unset.
func (s GuardedSetting) IsSet() bool {
	v := reflect.ValueOf(s.Value)
	if !v.IsValid() {
		return false
	}
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Map {
		return v.Len() > 0
	}
	return !v.IsZero()
}

// hash returns the hex SHA-256 of the setting's value, the same for every unset one.
func (s GuardedSetting) hash() string {
	var data []byte
	if s.IsSet() {
		// Config values are plain data and always marshal
		data, _ = json.Marshal(s.Value)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// GuardedSettings returns rc's keys that can reach the host, the network or other projects, in config file order.
func GuardedSettings(rc *AgenticRC) []GuardedSetting {
	return []GuardedSetting{
		{"root", rc.Root},
		{"namespace", rc.Namespace},
		{"docker_context", rc.DockerContext},
		{"marketplaces", rc.Marketplaces},
		{"build.custom_installs", rc.Build.CustomInstalls},
		{"run.extra_mounts", rc.Run.ExtraMounts},
		{"run.read_only_mounts", rc.Run.ReadOnlyMounts},
		{"run.secrets", rc.Run.Secrets},
		{"run.env", rc.Run.Env},
		{"run.proxy.enabled", rc.Run.Proxy.Enabled},
		{"run.proxy.mode", rc.Run.Proxy.Mode},
		{"run.proxy.allowed_hosts", rc.Run.Proxy.AllowedHosts},
		{CredentialsSetting, rc.Run.Proxy.Credentials},
		{"run.dind.enabled", rc.Run.Dind.Enabled},
	}
}
