package grpcadapter

import (
	"encoding/base64"

	"github.com/renbou/grpcbridge/internal/ascii"
	"google.golang.org/grpc/metadata"
)

const (
	// for backwards-compatibility with gRPC-Gateway
	grpcGatewayMetadataPrefix = "grpc-metadata-"
	metadataTimeout           = "grpc-timeout"
	metadataBinSuffix         = "-bin"
)

// ProxyMDFilterOpts specify the metadata fields to be forwarded in incoming headers (from a client) and outgoing headers/trailers (from a server).
// Additionally, prefix values can be specified for the appropriate forwarded fields to be prefixed with.
// For more context, see [ProxyMDFilter].
type ProxyMDFilterOpts struct {
	// AllowRequestMD is the allowlist for request metadata.
	AllowRequestMD []string
	// If PrefixRequestMD is specified, request metadata fields are prefixed with this value.
	PrefixRequestMD string
	// AllowResponseMD is the allowlist for response metadata.
	AllowResponseMD []string
	// If PrefixResponseMD is specified, response metadata fields are prefixed with this value.
	PrefixResponseMD string
	// AllowTrailerMD is the allowlist for response trailer metadata.
	AllowTrailerMD []string
	// If PrefixTrailerMD is specified, response trailer metadata fields are prefixed with this value.
	PrefixTrailerMD string
}

// ProxyMDFilter is the default implementation of [MetadataFilter] used by grpcbridge,
// suitable for use with various untrusted incoming requests.
// By default, no client or server metadata is forwarded, since both might contain sensitive fields:
// a client might send an X-Internal header that will be blindly trusted by a server,
// while a server might return an X-Internal header leaking some sensitive data.
//
// To enable metadata forwarding in one or the other direction, the field name must be specified
// in the appropriate allowlist in [ProxyMDFilterOpts]. Additionally, [ProxyMDFilter.FilterRequestMD]
// supports forwarding of the grpc-timeout value, since it will be terminated in grpcbridge by
// a [Forwarder], and as such is guaranteed to not affect the server in any way.
//
// For backwards compatibility with [gRPC-Gateway], ProxyMDFilter supports setting custom
// prefix values which will be used to prefix the appropriate metadata keys,
// i.e. IncomingPrefix and OutgoingPrefix can be set to "grpcgateway-" and "Grpc-Metadata-" for allowed fields
// to be proxied just like they would be with gRPC-Gateway. Prefixing incoming metadata with "Grpc-Metadata-"
// for it the prefix to be stripped is also supported, but such fields must also be explicitly allowed.
// For example, gRPC-Gateway would blindly forward Grpc-Metadata-Foo as Foo, but ProxyMDFilter
// would require "Grpc-Metadata-Foo" to be specified in AllowIncoming for that to happen.
//
// Binary metadata fields (fields whose keys are suffixed with "-bin") are decoded by
// [ProxyMDFilter.FilterRequestMD] for later encoding by the forwarding gRPC client.
//
// [gRPC-Gateway]: https://github.com/grpc-ecosystem/grpc-gateway
type ProxyMDFilter struct {
	opts ProxyMDFilterOpts
}

// NewProxyMDFilter creates a new [ProxyMDFilter] with the given options.
// See [ProxyMDFilter] and [ProxyMDFilterOpts] for more context.
func NewProxyMDFilter(opts ProxyMDFilterOpts) *ProxyMDFilter {
	return &ProxyMDFilter{
		opts: opts,
	}
}

// FilterRequestMD transforms incoming metadata according to the ProxyMDFilter configuration.
// If the metadata contains "grpc-timeout" fields, they will additionally be forwarded as-is.
func (pd *ProxyMDFilter) FilterRequestMD(md metadata.MD) metadata.MD {
	out := pd.filterRequest(md, pd.opts.AllowRequestMD, pd.opts.PrefixRequestMD)

	// Additionally support grpc-timeout for incoming headers.
	if v := md.Get(metadataTimeout); len(v) > 0 {
		out.Set(metadataTimeout, v...)
	}

	return out
}

// FilterResponseMD transforms outgoing metadata according to the ProxyMDFilter configuration.
// Outgoing fields are not modified in any way like they are in [ProxyMDFilter.FilterRequestMD],
// since they are intended to be read by a client as-is.
func (pd *ProxyMDFilter) FilterResponseMD(md metadata.MD) metadata.MD {
	return pd.filterResponse(md, pd.opts.AllowResponseMD, pd.opts.PrefixResponseMD)
}

// FilterTrailerMD transforms outgoing trailers according to the ProxyMDFilter configuration.
// It functions exactly like [ProxyMDFilter.FilterResponseMD], but uses the configuration for outgoing trailers.
func (pd *ProxyMDFilter) FilterTrailerMD(md metadata.MD) metadata.MD {
	return pd.filterResponse(md, pd.opts.AllowTrailerMD, pd.opts.PrefixTrailerMD)
}

func (pd *ProxyMDFilter) filterRequest(md metadata.MD, allow []string, prefix string) metadata.MD {
	out := make(metadata.MD)

	for _, k := range allow {
		v := md.Get(k)
		if len(v) < 1 {
			continue
		}

		if len(k) > len(grpcGatewayMetadataPrefix) && ascii.EqualFold(k[:len(grpcGatewayMetadataPrefix)], grpcGatewayMetadataPrefix) {
			k = k[len(grpcGatewayMetadataPrefix):]
		} else {
			k = prefix + k
		}

		// handle binary headers, since the gRPC client will re-encode them later.
		if len(k) > len(metadataBinSuffix) && ascii.EqualFold(k[len(k)-len(metadataBinSuffix):], metadataBinSuffix) {
			decoded := make([]string, 0, len(v))
			for _, s := range v {
				if b, err := decodeBinHeader(s); err == nil {
					decoded = append(decoded, string(b))
				}
			}
			v = decoded
		}

		out.Set(k, v...)
	}

	return out
}

func (pd *ProxyMDFilter) filterResponse(md metadata.MD, allow []string, prefix string) metadata.MD {
	out := make(metadata.MD)

	for _, k := range allow {
		if v := md.Get(k); len(v) > 0 {
			out.Set(prefix+k, v...)
		}
	}

	return out
}

// Performed as in gRPC-Gateway for compatibility.
// https://github.com/grpc-ecosystem/grpc-gateway/blob/17afee400c35e928b08998db1cd398af0b7ff371/runtime/context.go#L62
func decodeBinHeader(v string) ([]byte, error) {
	if len(v)%4 == 0 {
		// Input was padded, or padding was not necessary.
		return base64.StdEncoding.DecodeString(v)
	}

	return base64.RawStdEncoding.DecodeString(v)
}
