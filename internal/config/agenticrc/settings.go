package agenticrc

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

// GuardedSettings returns c's keys that can reach the host, the network or other projects, in config file order.
func (c *Config) GuardedSettings() []GuardedSetting {
	return []GuardedSetting{
		{"root", c.Root},
		{"namespace", c.Namespace},
		{"docker_context", c.DockerContext},
		{"marketplaces", c.Marketplaces},
		{"build.custom_installs", c.Build.CustomInstalls},
		{"run.mounts", c.Run.Mounts},
		{"run.env", c.Run.Env},
		{"run.ports", c.Run.Ports},
		{"run.args", c.Run.Args},
		{"run.runtime", c.Run.Runtime},
		{"run.proxy.enabled", c.Run.Proxy.Enabled},
		{"run.proxy.mode", c.Run.Proxy.Mode},
		{"run.proxy.backend", c.Run.Proxy.Backend},
		{"run.proxy.allowed_hosts", c.Run.Proxy.AllowedHosts},
		{"run.proxy.allowed_paths", c.Run.Proxy.AllowedPaths},
		{CredentialsSetting, c.Run.Proxy.Credentials},
		{"run.dind.enabled", c.Run.Dind.Enabled},
	}
}

// ApprovalHashes returns the hash of each of l's set guarded settings, the record of their approval.
func (l Layer) ApprovalHashes() map[string]string {
	hashes := make(map[string]string)
	for _, setting := range l.Config.GuardedSettings() {
		if setting.IsSet() {
			hashes[setting.Key] = setting.hash()
		}
	}
	return hashes
}

// ChangedSettings returns l's guarded settings that differ from the approved hashes; a missing key counts as approved unset.
func (l Layer) ChangedSettings(approved map[string]string) []GuardedSetting {
	var changed []GuardedSetting
	for _, setting := range l.Config.GuardedSettings() {
		want, ok := approved[setting.Key]
		if !ok {
			want = GuardedSetting{}.hash()
		}
		if setting.hash() != want {
			changed = append(changed, setting)
		}
	}
	return changed
}
