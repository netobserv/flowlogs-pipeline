package pipeline

import (
	"errors"
	"fmt"

	"github.com/netobserv/flowlogs-pipeline/pkg/api"
	"github.com/netobserv/flowlogs-pipeline/pkg/config"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/decode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/encode"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/extract"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/ingest"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/transform"
	"github.com/netobserv/flowlogs-pipeline/pkg/pipeline/write"
	"github.com/netobserv/gopipes/pkg/node"
	log "github.com/sirupsen/logrus"
)

// Error wraps any error caused by a wrong formation of the pipeline
type Error struct {
	StageName string
	wrapped   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("pipeline stage %q: %s", e.StageName, e.wrapped.Error())
}

func (e *Error) Unwrap() error {
	return e.wrapped
}

// builder stores the information that is only required during the build of the pipeline
type builder struct {
	pipelineStages   []*pipelineEntry
	configStages     []config.Stage
	configParams     []config.StageParam
	pipelineEntryMap map[string]*pipelineEntry
	createdStages    map[string]interface{}
	startNodes       []*node.Init
	terminalNodes    []*node.Terminal
}

type pipelineEntry struct {
	stageName   string
	stageType   string
	Ingester    ingest.Ingester
	Decoder     decode.Decoder
	Transformer transform.Transformer
	Extractor   extract.Extractor
	Encoder     encode.Encoder
	Writer      write.Writer
}

func newBuilder(params []config.StageParam, stages []config.Stage) *builder {
	return &builder{
		pipelineEntryMap: map[string]*pipelineEntry{},
		createdStages:    map[string]interface{}{},
		configStages:     stages,
		configParams:     params,
	}
}

// read the configuration stages definition and instantiate the corresponding native Go objects
func (b *builder) readStages() error {
	for _, param := range b.configParams {
		log.Debugf("stage = %v", param.Name)
		pEntry := pipelineEntry{
			stageName: param.Name,
			stageType: findStageType(&param),
		}
		var err error
		switch pEntry.stageType {
		case StageIngest:
			pEntry.Ingester, err = getIngester(param)
		case StageDecode:
			pEntry.Decoder, err = getDecoder(param)
		case StageTransform:
			pEntry.Transformer, err = getTransformer(param)
		case StageExtract:
			pEntry.Extractor, err = getExtractor(param)
		case StageEncode:
			pEntry.Encoder, err = getEncoder(param)
		case StageWrite:
			pEntry.Writer, err = getWriter(param)
		default:
			err = fmt.Errorf("invalid stage type: %v", pEntry.stageType)
		}
		if err != nil {
			return err
		}
		b.pipelineEntryMap[param.Name] = &pEntry
		b.pipelineStages = append(b.pipelineStages, &pEntry)
		log.Debugf("pipeline = %v", b.pipelineStages)
	}
	log.Debugf("pipeline = %v", b.pipelineStages)
	return nil
}

// reads the configured Go stages and connects between them
// readStages must be invoked before this
func (b *builder) build() (*Pipeline, error) {
	// accounts start and middle nodes that are connected to another node
	sendingNodes := map[string]struct{}{}
	// accounts middle or terminal nodes that receive data from another node
	receivingNodes := map[string]struct{}{}
	for _, connection := range b.configStages {
		if connection.Name == "" || connection.Follows == "" {
			// ignore entries that do not represent a connection
			continue
		}
		// instantiates (or loads from cache) the destination node of a connection
		dstEntry, ok := b.pipelineEntryMap[connection.Name]
		if !ok {
			return nil, fmt.Errorf("unknown pipeline stage: %s", connection.Name)
		}
		dstNode, err := b.getStageNode(dstEntry, connection.Name)
		if err != nil {
			return nil, err
		}
		dst, ok := dstNode.(node.Receiver)
		if !ok {
			return nil, fmt.Errorf("stage %q of type %q can't receive data",
				connection.Name, dstEntry.stageType)
		}
		// instantiates (or loads from cache) the source node of a connection
		srcEntry, ok := b.pipelineEntryMap[connection.Follows]
		if !ok {
			return nil, fmt.Errorf("unknown pipeline stage: %s", connection.Follows)
		}
		srcNode, err := b.getStageNode(srcEntry, connection.Follows)
		if err != nil {
			return nil, err
		}
		src, ok := srcNode.(node.Sender)
		if !ok {
			return nil, fmt.Errorf("stage %q of type %q can't send data",
				connection.Follows, srcEntry.stageType)
		}
		log.Infof("connecting stages: %s --> %s", connection.Follows, connection.Name)

		sendingNodes[connection.Follows] = struct{}{}
		receivingNodes[connection.Name] = struct{}{}
		// connects source and destination node, and catches any panic from the Go-Pipes library.
		var catchErr *Error
		func() {
			defer func() {
				if msg := recover(); msg != nil {
					catchErr = &Error{
						StageName: connection.Name,
						wrapped: fmt.Errorf("%q and %q stages haven't compatible input/outputs: %v",
							connection.Follows, connection.Name, msg),
					}
				}
			}()
			src.SendsTo(dst)
		}()
		if catchErr != nil {
			return nil, catchErr
		}
	}

	if err := b.verifyConnections(sendingNodes, receivingNodes); err != nil {
		return nil, err
	}
	if len(b.startNodes) == 0 {
		return nil, errors.New("no ingesters have been defined")
	}
	if len(b.terminalNodes) == 0 {
		return nil, errors.New("no writers have been defined")
	}
	return &Pipeline{
		startNodes:     b.startNodes,
		terminalNodes:  b.terminalNodes,
		pipelineStages: b.pipelineStages,
	}, nil
}

// verifies that all the start and middle nodes send data to another node
// verifies that all the middle and terminal nodes receive data from another node
func (b *builder) verifyConnections(sendingNodes, receivingNodes map[string]struct{}) error {
	for _, stg := range b.pipelineStages {
		if isReceptor(stg) {
			if _, ok := receivingNodes[stg.stageName]; !ok {
				return &Error{
					StageName: stg.stageName,
					wrapped: fmt.Errorf("pipeline stage from type %q"+
						" should receive data from at least another stage", stg.stageType),
				}
			}
		}
		if isSender(stg) {
			if _, ok := sendingNodes[stg.stageName]; !ok {
				return &Error{
					StageName: stg.stageName,
					wrapped: fmt.Errorf("pipeline stage from type %q"+
						" should send data to at least another stage", stg.stageType),
				}
			}
		}
	}
	return nil
}

func isReceptor(p *pipelineEntry) bool {
	return p.stageType != StageIngest
}

func isSender(p *pipelineEntry) bool {
	return p.stageType != StageWrite && p.stageType != StageEncode
}

func (b *builder) getStageNode(pe *pipelineEntry, stageID string) (interface{}, error) {
	if stg, ok := b.createdStages[stageID]; ok {
		return stg, nil
	}
	var stage interface{}
	// TODO: modify all the types' interfaces to not need to write loops here, the same
	// as we do with Ingest
	switch pe.stageType {
	case StageIngest:
		init := node.AsInit(pe.Ingester.Ingest)
		b.startNodes = append(b.startNodes, init)
		stage = init
	case StageWrite:
		term := node.AsTerminal(func(in <-chan []config.GenericMap) {
			for i := range in {
				pe.Writer.Write(i)
			}
		})
		b.terminalNodes = append(b.terminalNodes, term)
		stage = term
	case StageDecode:
		stage = node.AsMiddle(func(in <-chan []interface{}, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Decoder.Decode(i)
			}
		})
	case StageEncode:
		encode := node.AsTerminal(func(in <-chan []config.GenericMap) {
			for i := range in {
				pe.Encoder.Encode(i)
			}
		})
		b.terminalNodes = append(b.terminalNodes, encode)
		stage = encode
	case StageTransform:
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Transformer.Transform(i)
			}
		})
	case StageExtract:
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Extractor.Extract(i)
			}
		})
	default:
		return nil, &Error{
			StageName: stageID,
			wrapped:   fmt.Errorf("invalid stage type: %s", pe.stageType),
		}
	}
	b.createdStages[stageID] = stage
	return stage, nil
}

func getIngester(params config.StageParam) (ingest.Ingester, error) {
	var ingester ingest.Ingester
	var err error
	switch params.Ingest.Type {
	case api.FileType, api.FileLoopType, api.FileChunksType:
		ingester, err = ingest.NewIngestFile(params)
	case api.CollectorType:
		ingester, err = ingest.NewIngestCollector(params)
	case api.KafkaType:
		ingester, err = ingest.NewIngestKafka(params)
	case api.GRPCType:
		ingester, err = ingest.NewGRPCProtobuf(params)
	default:
		panic(fmt.Sprintf("`ingest` type %s not defined", params.Ingest.Type))
	}
	return ingester, err
}

func getDecoder(params config.StageParam) (decode.Decoder, error) {
	var decoder decode.Decoder
	var err error
	switch params.Decode.Type {
	case api.CastType:
		decoder, err = decode.NewDecodeCast()
	case api.JSONType:
		decoder, err = decode.NewDecodeJson()
	case api.AWSType:
		decoder, err = decode.NewDecodeAws(params)
	case api.PBType:
		decoder, err = decode.NewProtobuf()
	case api.NoneType:
		decoder, err = decode.NewDecodeNone()
	default:
		panic(fmt.Sprintf("`decode` type %s not defined; if no decoder needed, specify `none`", params.Decode.Type))
	}
	return decoder, err
}

func getWriter(params config.StageParam) (write.Writer, error) {
	var writer write.Writer
	var err error
	switch params.Write.Type {
	case api.StdoutType:
		writer, err = write.NewWriteStdout(params)
	case api.NoneType:
		writer, err = write.NewWriteNone()
	case api.LokiType:
		writer, err = write.NewWriteLoki(params)
	default:
		panic(fmt.Sprintf("`write` type %s not defined; if no writer needed, specify `none`", params.Write.Type))
	}
	return writer, err
}

func getTransformer(params config.StageParam) (transform.Transformer, error) {
	var transformer transform.Transformer
	var err error
	switch params.Transform.Type {
	case api.GenericType:
		transformer, err = transform.NewTransformGeneric(params)
	case api.FilterType:
		transformer, err = transform.NewTransformFilter(params)
	case api.NetworkType:
		transformer, err = transform.NewTransformNetwork(params)
	case api.NoneType:
		transformer, err = transform.NewTransformNone()
	default:
		panic(fmt.Sprintf("`transform` type %s not defined; if no transformer needed, specify `none`", params.Transform.Type))
	}
	return transformer, err
}

func getExtractor(params config.StageParam) (extract.Extractor, error) {
	var extractor extract.Extractor
	var err error
	switch params.Extract.Type {
	case "none":
		extractor, _ = extract.NewExtractNone()
	case "aggregates":
		extractor, err = extract.NewExtractAggregate(params)
	default:
		panic(fmt.Sprintf("`extract` type %s not defined; if no extractor needed, specify `none`", params.Extract.Type))
	}
	return extractor, err
}

func getEncoder(params config.StageParam) (encode.Encoder, error) {
	var encoder encode.Encoder
	var err error
	switch params.Encode.Type {
	case "prom":
		encoder, err = encode.NewEncodeProm(params)
	case "kafka":
		encoder, err = encode.NewEncodeKafka(params)
	case "none":
		encoder, _ = encode.NewEncodeNone()
	default:
		panic(fmt.Sprintf("`encode` type %s not defined; if no encoder needed, specify `none`", params.Encode.Type))
	}
	return encoder, err
}

// findStageParameters finds the matching config.param structure and identifies the stage type
func findStageType(param *config.StageParam) string {
	log.Debugf("findStageType: stage = %v", param.Name)
	if param.Ingest != nil && param.Ingest.Type != "" {
		return StageIngest
	}
	if param.Decode != nil && param.Decode.Type != "" {
		return StageDecode
	}
	if param.Transform != nil && param.Transform.Type != "" {
		return StageTransform
	}
	if param.Extract != nil && param.Extract.Type != "" {
		return StageExtract
	}
	if param.Encode != nil && param.Encode.Type != "" {
		return StageEncode
	}
	if param.Write != nil && param.Write.Type != "" {
		return StageWrite
	}
	return "unknown"
}
