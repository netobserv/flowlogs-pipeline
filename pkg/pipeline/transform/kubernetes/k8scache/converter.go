package k8scache

import (
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform/kubernetes/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// resourceEntryToMeta converts a gRPC ResourceEntry to the internal model used by the datasource.
func resourceEntryToMeta(entry *ResourceEntry) *model.ResourceMetaData {
	if entry == nil {
		return nil
	}
	meta := &model.ResourceMetaData{
		ObjectMeta: metav1.ObjectMeta{
			Name:            entry.Name,
			Namespace:       entry.Namespace,
			UID:             "",
			ResourceVersion: entry.ResourceVersion,
			Labels:          entry.Labels,
			Annotations:     entry.Annotations,
		},
		Kind:             entry.Kind,
		OwnerName:        entry.OwnerName,
		OwnerKind:        entry.OwnerKind,
		HostName:         entry.HostName,
		HostIP:           entry.HostIp,
		NetworkName:      entry.NetworkName,
		IPs:              append([]string(nil), entry.Ips...),
		SecondaryNetKeys: append([]string(nil), entry.SecondaryNetKeys...),
	}
	if entry.Uid != "" {
		meta.UID = types.UID(entry.Uid)
	}
	if entry.CreationTimestamp != 0 {
		meta.CreationTimestamp = metav1.Unix(entry.CreationTimestamp, 0)
	}
	return meta
}

// resourceEntriesToMeta converts a slice of ResourceEntry to model.ResourceMetaData.
func resourceEntriesToMeta(entries []*ResourceEntry) []*model.ResourceMetaData {
	if len(entries) == 0 {
		return nil
	}
	out := make([]*model.ResourceMetaData, 0, len(entries))
	for _, e := range entries {
		if m := resourceEntryToMeta(e); m != nil {
			out = append(out, m)
		}
	}
	return out
}

// metaToResourceEntry converts internal model.ResourceMetaData to gRPC ResourceEntry.
func metaToResourceEntry(meta *model.ResourceMetaData) *ResourceEntry {
	if meta == nil {
		return nil
	}
	entry := &ResourceEntry{
		Kind:             meta.Kind,
		Namespace:        meta.Namespace,
		Name:             meta.Name,
		Uid:              string(meta.UID),
		OwnerName:        meta.OwnerName,
		OwnerKind:        meta.OwnerKind,
		HostName:         meta.HostName,
		HostIp:           meta.HostIP,
		NetworkName:      meta.NetworkName,
		Ips:              append([]string(nil), meta.IPs...),
		SecondaryNetKeys: append([]string(nil), meta.SecondaryNetKeys...),
		Labels:           meta.Labels,
		Annotations:      meta.Annotations,
		ResourceVersion:  meta.ResourceVersion,
	}
	if !meta.CreationTimestamp.IsZero() {
		entry.CreationTimestamp = meta.CreationTimestamp.Unix()
	}
	return entry
}

// metaToResourceEntries converts a slice of model.ResourceMetaData to ResourceEntry.
func metaToResourceEntries(metas []*model.ResourceMetaData) []*ResourceEntry {
	if len(metas) == 0 {
		return nil
	}
	out := make([]*ResourceEntry, 0, len(metas))
	for _, m := range metas {
		if e := metaToResourceEntry(m); e != nil {
			out = append(out, e)
		}
	}
	return out
}
