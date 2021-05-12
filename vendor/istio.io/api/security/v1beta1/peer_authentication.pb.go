// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: security/v1beta1/peer_authentication.proto

package v1beta1

import (
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	v1beta1 "istio.io/api/type/v1beta1"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type PeerAuthentication_MutualTLS_Mode int32

const (
	// Inherit from parent, if has one. Otherwise treated as PERMISSIVE.
	PeerAuthentication_MutualTLS_UNSET PeerAuthentication_MutualTLS_Mode = 0
	// Connection is not tunneled.
	PeerAuthentication_MutualTLS_DISABLE PeerAuthentication_MutualTLS_Mode = 1
	// Connection can be either plaintext or mTLS tunnel.
	PeerAuthentication_MutualTLS_PERMISSIVE PeerAuthentication_MutualTLS_Mode = 2
	// Connection is an mTLS tunnel (TLS with client cert must be presented).
	PeerAuthentication_MutualTLS_STRICT PeerAuthentication_MutualTLS_Mode = 3
)

var PeerAuthentication_MutualTLS_Mode_name = map[int32]string{
	0: "UNSET",
	1: "DISABLE",
	2: "PERMISSIVE",
	3: "STRICT",
}

var PeerAuthentication_MutualTLS_Mode_value = map[string]int32{
	"UNSET":      0,
	"DISABLE":    1,
	"PERMISSIVE": 2,
	"STRICT":     3,
}

func (x PeerAuthentication_MutualTLS_Mode) String() string {
	return proto.EnumName(PeerAuthentication_MutualTLS_Mode_name, int32(x))
}

func (PeerAuthentication_MutualTLS_Mode) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_59c7062c50455f33, []int{0, 0, 0}
}

// PeerAuthentication defines how traffic will be tunneled (or not) to the sidecar.
//
// Examples:
//
// Policy to allow mTLS traffic for all workloads under namespace `foo`:
// ```yaml
// apiVersion: security.istio.io/v1beta1
// kind: PeerAuthentication
// metadata:
//   name: default
//   namespace: foo
// spec:
//   mtls:
//     mode: STRICT
// ```
// For mesh level, put the policy in root-namespace according to your Istio installation.
//
// Policies to allow both mTLS & plaintext traffic for all workloads under namespace `foo`, but
// require mTLS for workload `finance`.
// ```yaml
// apiVersion: security.istio.io/v1beta1
// kind: PeerAuthentication
// metadata:
//   name: default
//   namespace: foo
// spec:
//   mtls:
//     mode: PERMISSIVE
// ---
// apiVersion: security.istio.io/v1beta1
// kind: PeerAuthentication
// metadata:
//   name: default
//   namespace: foo
// spec:
//   selector:
//     matchLabels:
//       app: finance
//   mtls:
//     mode: STRICT
// ```
// Policy to allow mTLS strict for all workloads, but leave port 8080 to
// plaintext:
// ```yaml
// apiVersion: security.istio.io/v1beta1
// kind: PeerAuthentication
// metadata:
//   name: default
//   namespace: foo
// spec:
//   selector:
//     matchLabels:
//       app: finance
//   mtls:
//     mode: STRICT
//   portLevelMtls:
//     8080:
//       mode: DISABLE
// ```
// Policy to inherit mTLS mode from namespace (or mesh) settings, and overwrite
// settings for port 8080
// ```yaml
// apiVersion: security.istio.io/v1beta1
// kind: PeerAuthentication
// metadata:
//   name: default
//   namespace: foo
// spec:
//   selector:
//     matchLabels:
//       app: finance
//   mtls:
//     mode: UNSET
//   portLevelMtls:
//     8080:
//       mode: DISABLE
// ```
//
// <!-- crd generation tags
// +cue-gen:PeerAuthentication:groupName:security.istio.io
// +cue-gen:PeerAuthentication:version:v1beta1
// +cue-gen:PeerAuthentication:storageVersion
// +cue-gen:PeerAuthentication:annotations:helm.sh/resource-policy=keep
// +cue-gen:PeerAuthentication:labels:app=istio-pilot,chart=istio,istio=security,heritage=Tiller,release=istio
// +cue-gen:PeerAuthentication:subresource:status
// +cue-gen:PeerAuthentication:scope:Namespaced
// +cue-gen:PeerAuthentication:resource:categories=istio-io,security-istio-io,shortNames=pa
// +cue-gen:PeerAuthentication:preserveUnknownFields:false
// +cue-gen:PeerAuthentication:printerColumn:name=Mode,type=string,JSONPath=.spec.mtls.mode,description="Defines the mTLS mode used for peer authentication."
// +cue-gen:PeerAuthentication:printerColumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp,description="CreationTimestamp is a timestamp
// representing the server time when this object was created. It is not guaranteed to be set in happens-before order across separate operations.
// Clients may not set this value. It is represented in RFC3339 form and is in UTC.
// Populated by the system. Read-only. Null for lists. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
// -->
//
// <!-- go code generation tags
// +kubetype-gen
// +kubetype-gen:groupVersion=security.istio.io/v1beta1
// +genclient
// +k8s:deepcopy-gen=true
// -->
type PeerAuthentication struct {
	// The selector determines the workloads to apply the ChannelAuthentication on.
	// If not set, the policy will be applied to all workloads in the same namespace as the policy.
	Selector *v1beta1.WorkloadSelector `protobuf:"bytes,1,opt,name=selector,proto3" json:"selector,omitempty"`
	// Mutual TLS settings for workload. If not defined, inherit from parent.
	Mtls *PeerAuthentication_MutualTLS `protobuf:"bytes,2,opt,name=mtls,proto3" json:"mtls,omitempty"`
	// Port specific mutual TLS settings.
	PortLevelMtls        map[uint32]*PeerAuthentication_MutualTLS `protobuf:"bytes,3,rep,name=port_level_mtls,json=portLevelMtls,proto3" json:"port_level_mtls,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}                                 `json:"-"`
	XXX_unrecognized     []byte                                   `json:"-"`
	XXX_sizecache        int32                                    `json:"-"`
}

func (m *PeerAuthentication) Reset()         { *m = PeerAuthentication{} }
func (m *PeerAuthentication) String() string { return proto.CompactTextString(m) }
func (*PeerAuthentication) ProtoMessage()    {}
func (*PeerAuthentication) Descriptor() ([]byte, []int) {
	return fileDescriptor_59c7062c50455f33, []int{0}
}
func (m *PeerAuthentication) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PeerAuthentication) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PeerAuthentication.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PeerAuthentication) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PeerAuthentication.Merge(m, src)
}
func (m *PeerAuthentication) XXX_Size() int {
	return m.Size()
}
func (m *PeerAuthentication) XXX_DiscardUnknown() {
	xxx_messageInfo_PeerAuthentication.DiscardUnknown(m)
}

var xxx_messageInfo_PeerAuthentication proto.InternalMessageInfo

func (m *PeerAuthentication) GetSelector() *v1beta1.WorkloadSelector {
	if m != nil {
		return m.Selector
	}
	return nil
}

func (m *PeerAuthentication) GetMtls() *PeerAuthentication_MutualTLS {
	if m != nil {
		return m.Mtls
	}
	return nil
}

func (m *PeerAuthentication) GetPortLevelMtls() map[uint32]*PeerAuthentication_MutualTLS {
	if m != nil {
		return m.PortLevelMtls
	}
	return nil
}

// Mutual TLS settings.
type PeerAuthentication_MutualTLS struct {
	// Defines the mTLS mode used for peer authentication.
	Mode                 PeerAuthentication_MutualTLS_Mode `protobuf:"varint,1,opt,name=mode,proto3,enum=istio.security.v1beta1.PeerAuthentication_MutualTLS_Mode" json:"mode,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                          `json:"-"`
	XXX_unrecognized     []byte                            `json:"-"`
	XXX_sizecache        int32                             `json:"-"`
}

func (m *PeerAuthentication_MutualTLS) Reset()         { *m = PeerAuthentication_MutualTLS{} }
func (m *PeerAuthentication_MutualTLS) String() string { return proto.CompactTextString(m) }
func (*PeerAuthentication_MutualTLS) ProtoMessage()    {}
func (*PeerAuthentication_MutualTLS) Descriptor() ([]byte, []int) {
	return fileDescriptor_59c7062c50455f33, []int{0, 0}
}
func (m *PeerAuthentication_MutualTLS) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PeerAuthentication_MutualTLS) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PeerAuthentication_MutualTLS.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PeerAuthentication_MutualTLS) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PeerAuthentication_MutualTLS.Merge(m, src)
}
func (m *PeerAuthentication_MutualTLS) XXX_Size() int {
	return m.Size()
}
func (m *PeerAuthentication_MutualTLS) XXX_DiscardUnknown() {
	xxx_messageInfo_PeerAuthentication_MutualTLS.DiscardUnknown(m)
}

var xxx_messageInfo_PeerAuthentication_MutualTLS proto.InternalMessageInfo

func (m *PeerAuthentication_MutualTLS) GetMode() PeerAuthentication_MutualTLS_Mode {
	if m != nil {
		return m.Mode
	}
	return PeerAuthentication_MutualTLS_UNSET
}

func init() {
	proto.RegisterEnum("istio.security.v1beta1.PeerAuthentication_MutualTLS_Mode", PeerAuthentication_MutualTLS_Mode_name, PeerAuthentication_MutualTLS_Mode_value)
	proto.RegisterType((*PeerAuthentication)(nil), "istio.security.v1beta1.PeerAuthentication")
	proto.RegisterMapType((map[uint32]*PeerAuthentication_MutualTLS)(nil), "istio.security.v1beta1.PeerAuthentication.PortLevelMtlsEntry")
	proto.RegisterType((*PeerAuthentication_MutualTLS)(nil), "istio.security.v1beta1.PeerAuthentication.MutualTLS")
}

func init() {
	proto.RegisterFile("security/v1beta1/peer_authentication.proto", fileDescriptor_59c7062c50455f33)
}

var fileDescriptor_59c7062c50455f33 = []byte{
	// 380 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xa4, 0x92, 0x41, 0x8b, 0xda, 0x40,
	0x14, 0xc7, 0x3b, 0x26, 0xda, 0xfa, 0x44, 0x1b, 0xe6, 0x50, 0xc4, 0x52, 0x11, 0xe9, 0x41, 0x5a,
	0x98, 0xa0, 0xed, 0xa1, 0x15, 0x0a, 0xd5, 0x36, 0xd0, 0x14, 0x53, 0x24, 0x49, 0x5b, 0xe8, 0x45,
	0xa2, 0x3e, 0x68, 0x70, 0x74, 0xc2, 0x64, 0x12, 0xc8, 0x17, 0x29, 0xfd, 0x48, 0x3d, 0xf6, 0x23,
	0x2c, 0x7e, 0x92, 0x25, 0x89, 0xca, 0xee, 0xba, 0x97, 0xdd, 0xbd, 0xcd, 0x0c, 0xef, 0xf7, 0x7b,
	0xff, 0x79, 0x3c, 0x78, 0x15, 0xe3, 0x2a, 0x91, 0xa1, 0xca, 0xcc, 0x74, 0xb8, 0x44, 0x15, 0x0c,
	0xcd, 0x08, 0x51, 0x2e, 0x82, 0x44, 0xfd, 0xc6, 0x9d, 0x0a, 0x57, 0x81, 0x0a, 0xc5, 0x8e, 0x45,
	0x52, 0x28, 0x41, 0x9f, 0x85, 0xb1, 0x0a, 0x05, 0x3b, 0x12, 0xec, 0x40, 0x74, 0x9e, 0xab, 0x2c,
	0xc2, 0x13, 0x1f, 0x23, 0xc7, 0x95, 0x12, 0xb2, 0x84, 0xfa, 0x7f, 0x75, 0xa0, 0x73, 0x44, 0x39,
	0xb9, 0x66, 0xa4, 0x1f, 0xe1, 0xc9, 0xb1, 0xb0, 0x4d, 0x7a, 0x64, 0xd0, 0x18, 0xbd, 0x64, 0xa5,
	0x3e, 0x97, 0x1d, 0xd5, 0xec, 0xa7, 0x90, 0x1b, 0x2e, 0x82, 0xb5, 0x77, 0xa8, 0x75, 0x4f, 0x14,
	0xfd, 0x02, 0xfa, 0x56, 0xf1, 0xb8, 0x5d, 0x29, 0xe8, 0xb7, 0xec, 0xf6, 0x70, 0xec, 0xbc, 0x37,
	0x73, 0x12, 0x95, 0x04, 0xdc, 0x9f, 0x79, 0x6e, 0x61, 0xa0, 0x08, 0x4f, 0x23, 0x21, 0xd5, 0x82,
	0x63, 0x8a, 0x7c, 0x51, 0x48, 0xb5, 0x9e, 0x36, 0x68, 0x8c, 0x3e, 0xdc, 0x41, 0x3a, 0x17, 0x52,
	0xcd, 0x72, 0x81, 0xa3, 0x78, 0x6c, 0xed, 0x94, 0xcc, 0xdc, 0x66, 0x74, 0xf5, 0xad, 0xf3, 0x87,
	0x40, 0xfd, 0xd4, 0x9a, 0x3a, 0xa0, 0x6f, 0xc5, 0x1a, 0x8b, 0xcf, 0xb7, 0x46, 0xef, 0xef, 0x13,
	0x9f, 0x39, 0x62, 0x8d, 0x6e, 0xa1, 0xe9, 0x8f, 0x41, 0xcf, 0x6f, 0xb4, 0x0e, 0xd5, 0xef, 0xdf,
	0x3c, 0xcb, 0x37, 0x1e, 0xd1, 0x06, 0x3c, 0xfe, 0x6c, 0x7b, 0x93, 0xe9, 0xcc, 0x32, 0x08, 0x6d,
	0x01, 0xcc, 0x2d, 0xd7, 0xb1, 0x3d, 0xcf, 0xfe, 0x61, 0x19, 0x15, 0x0a, 0x50, 0xf3, 0x7c, 0xd7,
	0xfe, 0xe4, 0x1b, 0x5a, 0x27, 0x05, 0x7a, 0x9e, 0x9e, 0x1a, 0xa0, 0x6d, 0x30, 0x2b, 0xf2, 0x35,
	0xdd, 0xfc, 0x48, 0xbf, 0x42, 0x35, 0x0d, 0x78, 0x82, 0x0f, 0x1a, 0x79, 0xa9, 0x18, 0x57, 0xde,
	0x91, 0xe9, 0xeb, 0x7f, 0xfb, 0x2e, 0xf9, 0xbf, 0xef, 0x92, 0x8b, 0x7d, 0x97, 0xfc, 0x7a, 0x51,
	0xda, 0x42, 0x61, 0x06, 0x51, 0x68, 0xde, 0x5c, 0xcb, 0x65, 0xad, 0x58, 0xa7, 0x37, 0x97, 0x01,
	0x00, 0x00, 0xff, 0xff, 0x24, 0x64, 0x92, 0x25, 0xb1, 0x02, 0x00, 0x00,
}

func (m *PeerAuthentication) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PeerAuthentication) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PeerAuthentication) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if len(m.PortLevelMtls) > 0 {
		for k := range m.PortLevelMtls {
			v := m.PortLevelMtls[k]
			baseI := i
			if v != nil {
				{
					size, err := v.MarshalToSizedBuffer(dAtA[:i])
					if err != nil {
						return 0, err
					}
					i -= size
					i = encodeVarintPeerAuthentication(dAtA, i, uint64(size))
				}
				i--
				dAtA[i] = 0x12
			}
			i = encodeVarintPeerAuthentication(dAtA, i, uint64(k))
			i--
			dAtA[i] = 0x8
			i = encodeVarintPeerAuthentication(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0x1a
		}
	}
	if m.Mtls != nil {
		{
			size, err := m.Mtls.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintPeerAuthentication(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	if m.Selector != nil {
		{
			size, err := m.Selector.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintPeerAuthentication(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *PeerAuthentication_MutualTLS) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PeerAuthentication_MutualTLS) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PeerAuthentication_MutualTLS) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Mode != 0 {
		i = encodeVarintPeerAuthentication(dAtA, i, uint64(m.Mode))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintPeerAuthentication(dAtA []byte, offset int, v uint64) int {
	offset -= sovPeerAuthentication(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *PeerAuthentication) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Selector != nil {
		l = m.Selector.Size()
		n += 1 + l + sovPeerAuthentication(uint64(l))
	}
	if m.Mtls != nil {
		l = m.Mtls.Size()
		n += 1 + l + sovPeerAuthentication(uint64(l))
	}
	if len(m.PortLevelMtls) > 0 {
		for k, v := range m.PortLevelMtls {
			_ = k
			_ = v
			l = 0
			if v != nil {
				l = v.Size()
				l += 1 + sovPeerAuthentication(uint64(l))
			}
			mapEntrySize := 1 + sovPeerAuthentication(uint64(k)) + l
			n += mapEntrySize + 1 + sovPeerAuthentication(uint64(mapEntrySize))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *PeerAuthentication_MutualTLS) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Mode != 0 {
		n += 1 + sovPeerAuthentication(uint64(m.Mode))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovPeerAuthentication(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozPeerAuthentication(x uint64) (n int) {
	return sovPeerAuthentication(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *PeerAuthentication) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowPeerAuthentication
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: PeerAuthentication: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PeerAuthentication: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Selector", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowPeerAuthentication
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Selector == nil {
				m.Selector = &v1beta1.WorkloadSelector{}
			}
			if err := m.Selector.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Mtls", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowPeerAuthentication
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Mtls == nil {
				m.Mtls = &PeerAuthentication_MutualTLS{}
			}
			if err := m.Mtls.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PortLevelMtls", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowPeerAuthentication
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.PortLevelMtls == nil {
				m.PortLevelMtls = make(map[uint32]*PeerAuthentication_MutualTLS)
			}
			var mapkey uint32
			var mapvalue *PeerAuthentication_MutualTLS
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowPeerAuthentication
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					wire |= uint64(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				fieldNum := int32(wire >> 3)
				if fieldNum == 1 {
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowPeerAuthentication
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						mapkey |= uint32(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
				} else if fieldNum == 2 {
					var mapmsglen int
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowPeerAuthentication
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						mapmsglen |= int(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					if mapmsglen < 0 {
						return ErrInvalidLengthPeerAuthentication
					}
					postmsgIndex := iNdEx + mapmsglen
					if postmsgIndex < 0 {
						return ErrInvalidLengthPeerAuthentication
					}
					if postmsgIndex > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = &PeerAuthentication_MutualTLS{}
					if err := mapvalue.Unmarshal(dAtA[iNdEx:postmsgIndex]); err != nil {
						return err
					}
					iNdEx = postmsgIndex
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipPeerAuthentication(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if (skippy < 0) || (iNdEx+skippy) < 0 {
						return ErrInvalidLengthPeerAuthentication
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.PortLevelMtls[mapkey] = mapvalue
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipPeerAuthentication(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *PeerAuthentication_MutualTLS) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowPeerAuthentication
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MutualTLS: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MutualTLS: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Mode", wireType)
			}
			m.Mode = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowPeerAuthentication
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Mode |= PeerAuthentication_MutualTLS_Mode(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipPeerAuthentication(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthPeerAuthentication
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipPeerAuthentication(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowPeerAuthentication
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowPeerAuthentication
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowPeerAuthentication
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthPeerAuthentication
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupPeerAuthentication
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthPeerAuthentication
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthPeerAuthentication        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowPeerAuthentication          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupPeerAuthentication = fmt.Errorf("proto: unexpected end of group")
)
