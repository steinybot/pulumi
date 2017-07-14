// *** WARNING: this file was generated by the Lumi IDL Compiler (LUMIDL). ***
// *** Do not edit by hand unless you're certain you know what you are doing! ***

package cloudwatch

import (
    "errors"

    pbempty "github.com/golang/protobuf/ptypes/empty"
    pbstruct "github.com/golang/protobuf/ptypes/struct"
    "golang.org/x/net/context"

    "github.com/pulumi/lumi/pkg/resource"
    "github.com/pulumi/lumi/pkg/resource/plugin"
    "github.com/pulumi/lumi/pkg/tokens"
    "github.com/pulumi/lumi/pkg/util/contract"
    "github.com/pulumi/lumi/pkg/util/mapper"
    "github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
)

/* RPC stubs for LogGroup resource provider */

// LogGroupToken is the type token corresponding to the LogGroup package type.
const LogGroupToken = tokens.Type("aws:cloudwatch/logGroup:LogGroup")

// LogGroupProviderOps is a pluggable interface for LogGroup-related management functionality.
type LogGroupProviderOps interface {
    Check(ctx context.Context, obj *LogGroup, property string) error
    Create(ctx context.Context, obj *LogGroup) (resource.ID, error)
    Get(ctx context.Context, id resource.ID) (*LogGroup, error)
    InspectChange(ctx context.Context,
        id resource.ID, old *LogGroup, new *LogGroup, diff *resource.ObjectDiff) ([]string, error)
    Update(ctx context.Context,
        id resource.ID, old *LogGroup, new *LogGroup, diff *resource.ObjectDiff) error
    Delete(ctx context.Context, id resource.ID) error
}

// LogGroupProvider is a dynamic gRPC-based plugin for managing LogGroup resources.
type LogGroupProvider struct {
    ops LogGroupProviderOps
}

// NewLogGroupProvider allocates a resource provider that delegates to a ops instance.
func NewLogGroupProvider(ops LogGroupProviderOps) lumirpc.ResourceProviderServer {
    contract.Assert(ops != nil)
    return &LogGroupProvider{ops: ops}
}

func (p *LogGroupProvider) Check(
    ctx context.Context, req *lumirpc.CheckRequest) (*lumirpc.CheckResponse, error) {
    contract.Assert(req.GetType() == string(LogGroupToken))
    obj, _, err := p.Unmarshal(req.GetProperties())
    if err != nil {
        return plugin.NewCheckResponse(err), nil
    }
    var failures []error
    if failure := p.ops.Check(ctx, obj, ""); failure != nil {
        failures = append(failures, failure)
    }
    unks := req.GetUnknowns()
    if !unks["name"] {
        if failure := p.ops.Check(ctx, obj, "name"); failure != nil {
            failures = append(failures,
                resource.NewPropertyError("LogGroup", "name", failure))
        }
    }
    if !unks["logGroupName"] {
        if failure := p.ops.Check(ctx, obj, "logGroupName"); failure != nil {
            failures = append(failures,
                resource.NewPropertyError("LogGroup", "logGroupName", failure))
        }
    }
    if !unks["retentionInDays"] {
        if failure := p.ops.Check(ctx, obj, "retentionInDays"); failure != nil {
            failures = append(failures,
                resource.NewPropertyError("LogGroup", "retentionInDays", failure))
        }
    }
    if len(failures) > 0 {
        return plugin.NewCheckResponse(resource.NewErrors(failures)), nil
    }
    return plugin.NewCheckResponse(nil), nil
}

func (p *LogGroupProvider) Name(
    ctx context.Context, req *lumirpc.NameRequest) (*lumirpc.NameResponse, error) {
    contract.Assert(req.GetType() == string(LogGroupToken))
    obj, _, err := p.Unmarshal(req.GetProperties())
    if err != nil {
        return nil, err
    }
    if obj.Name == nil || *obj.Name == "" {
        if req.Unknowns[LogGroup_Name] {
            return nil, errors.New("Name property cannot be computed from unknown outputs")
        }
        return nil, errors.New("Name property cannot be empty")
    }
    return &lumirpc.NameResponse{Name: *obj.Name}, nil
}

func (p *LogGroupProvider) Create(
    ctx context.Context, req *lumirpc.CreateRequest) (*lumirpc.CreateResponse, error) {
    contract.Assert(req.GetType() == string(LogGroupToken))
    obj, _, err := p.Unmarshal(req.GetProperties())
    if err != nil {
        return nil, err
    }
    id, err := p.ops.Create(ctx, obj)
    if err != nil {
        return nil, err
    }
    return &lumirpc.CreateResponse{Id: string(id)}, nil
}

func (p *LogGroupProvider) Get(
    ctx context.Context, req *lumirpc.GetRequest) (*lumirpc.GetResponse, error) {
    contract.Assert(req.GetType() == string(LogGroupToken))
    id := resource.ID(req.GetId())
    obj, err := p.ops.Get(ctx, id)
    if err != nil {
        return nil, err
    }
    return &lumirpc.GetResponse{
        Properties: plugin.MarshalProperties(
            resource.NewPropertyMap(obj), plugin.MarshalOptions{}),
    }, nil
}

func (p *LogGroupProvider) InspectChange(
    ctx context.Context, req *lumirpc.InspectChangeRequest) (*lumirpc.InspectChangeResponse, error) {
    contract.Assert(req.GetType() == string(LogGroupToken))
    id := resource.ID(req.GetId())
    old, oldprops, err := p.Unmarshal(req.GetOlds())
    if err != nil {
        return nil, err
    }
    new, newprops, err := p.Unmarshal(req.GetNews())
    if err != nil {
        return nil, err
    }
    var replaces []string
    diff := oldprops.Diff(newprops)
    if diff != nil {
        if diff.Changed("name") {
            replaces = append(replaces, "name")
        }
        if diff.Changed("logGroupName") {
            replaces = append(replaces, "logGroupName")
        }
    }
    more, err := p.ops.InspectChange(ctx, id, old, new, diff)
    if err != nil {
        return nil, err
    }
    return &lumirpc.InspectChangeResponse{
        Replaces: append(replaces, more...),
    }, err
}

func (p *LogGroupProvider) Update(
    ctx context.Context, req *lumirpc.UpdateRequest) (*pbempty.Empty, error) {
    contract.Assert(req.GetType() == string(LogGroupToken))
    id := resource.ID(req.GetId())
    old, oldprops, err := p.Unmarshal(req.GetOlds())
    if err != nil {
        return nil, err
    }
    new, newprops, err := p.Unmarshal(req.GetNews())
    if err != nil {
        return nil, err
    }
    diff := oldprops.Diff(newprops)
    if err := p.ops.Update(ctx, id, old, new, diff); err != nil {
        return nil, err
    }
    return &pbempty.Empty{}, nil
}

func (p *LogGroupProvider) Delete(
    ctx context.Context, req *lumirpc.DeleteRequest) (*pbempty.Empty, error) {
    contract.Assert(req.GetType() == string(LogGroupToken))
    id := resource.ID(req.GetId())
    if err := p.ops.Delete(ctx, id); err != nil {
        return nil, err
    }
    return &pbempty.Empty{}, nil
}

func (p *LogGroupProvider) Unmarshal(
    v *pbstruct.Struct) (*LogGroup, resource.PropertyMap, error) {
    var obj LogGroup
    props := plugin.UnmarshalProperties(v, plugin.MarshalOptions{RawResources: true})
    return &obj, props, mapper.MapIU(props.Mappable(), &obj)
}

/* Marshalable LogGroup structure(s) */

// LogGroup is a marshalable representation of its corresponding IDL type.
type LogGroup struct {
    Name *string `lumi:"name,optional"`
    LogGroupName *string `lumi:"logGroupName,optional"`
    RetentionInDays *float64 `lumi:"retentionInDays,optional"`
}

// LogGroup's properties have constants to make dealing with diffs and property bags easier.
const (
    LogGroup_Name = "name"
    LogGroup_LogGroupName = "logGroupName"
    LogGroup_RetentionInDays = "retentionInDays"
)


