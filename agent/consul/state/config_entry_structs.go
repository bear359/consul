package state

import (
	"github.com/hashicorp/consul/agent/structs"
)

// TODO: rename
type ServiceConfigResponseTHREE struct {
	MaxIndex uint64

	ServiceDefaults map[structs.ServiceID]*structs.ServiceConfigEntry
	ProxyDefaults   map[string]*structs.ProxyConfigEntry

	Signature ConfigEntryIndexSignature
}

func (r *ServiceConfigResponseTHREE) AcceptMaxIndex(v uint64) {
	if v > r.MaxIndex {
		r.MaxIndex = v
	}
}

func (r *ServiceConfigResponseTHREE) AcceptServiceDefaults(
	name string,
	entMeta *structs.EnterpriseMeta,
	entry *structs.ServiceConfigEntry,
) {
	ckn := NewConfigEntryKindName(structs.ServiceDefaults, name, entMeta)
	if entry != nil {
		r.AddServiceDefaults(entry)
		r.Signature.Add(ckn, entry.ModifyIndex)
	} else {
		r.Signature.Add(ckn, 0)
	}
}

func (r *ServiceConfigResponseTHREE) AcceptProxyDefaults(
	entMeta *structs.EnterpriseMeta,
	entry *structs.ProxyConfigEntry,
) {
	ckn := NewConfigEntryKindName(structs.ProxyDefaults, structs.ProxyConfigGlobal, entMeta)
	if entry != nil {
		r.AddProxyDefaults(entry)
		r.Signature.Add(ckn, entry.ModifyIndex)
	} else {
		r.Signature.Add(ckn, 0)
	}
}

func (r *ServiceConfigResponseTHREE) AddServiceDefaults(entry *structs.ServiceConfigEntry) {
	if entry == nil {
		return
	}
	if r.ServiceDefaults == nil {
		r.ServiceDefaults = make(map[structs.ServiceID]*structs.ServiceConfigEntry)
	}

	sid := structs.NewServiceID(entry.Name, &entry.EnterpriseMeta)
	r.ServiceDefaults[sid] = entry
}

func (r *ServiceConfigResponseTHREE) AddProxyDefaults(entry *structs.ProxyConfigEntry) {
	if entry == nil {
		return
	}
	if r.ProxyDefaults == nil {
		r.ProxyDefaults = make(map[string]*structs.ProxyConfigEntry)
	}

	r.ProxyDefaults[entry.PartitionOrDefault()] = entry
}

func (r *ServiceConfigResponseTHREE) GetServiceDefaults(sid structs.ServiceID) *structs.ServiceConfigEntry {
	if r.ServiceDefaults == nil {
		return nil
	}
	return r.ServiceDefaults[sid]
}

func (r *ServiceConfigResponseTHREE) GetProxyDefaults(partition string) *structs.ProxyConfigEntry {
	if r.ProxyDefaults == nil {
		return nil
	}
	return r.ProxyDefaults[partition]
}

type ConfigEntryIndexSignature struct {
	// Items maps the config entries that were requested to the modify index of
	// the returned elements OR zero if it does not exist.
	Items map[ConfigEntryKindName]uint64
}

func (s *ConfigEntryIndexSignature) IsZero() bool {
	return s.Items == nil
}

func (s *ConfigEntryIndexSignature) Add(ckn ConfigEntryKindName, index uint64) {
	if s.Items == nil {
		s.Items = make(map[ConfigEntryKindName]uint64)
	}
	s.Items[ckn] = index
}

func (s *ConfigEntryIndexSignature) IsSame(other *ConfigEntryIndexSignature) bool {
	if len(s.Items) != len(other.Items) {
		return false
	}

	for k, v := range s.Items {
		otherV, ok := other.Items[k]
		if !ok {
			return false
		} else if otherV != v {
			return false
		}
	}
	return true
}
