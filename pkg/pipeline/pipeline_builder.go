package pipeline

import (
	"errors"
	"fmt"

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
	configStage config.Stage
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
	for _, stage := range b.configStages {
		log.Debugf("stage = %v", stage)
		params, stageType := findStageParameters(stage, b.configParams)
		if params == nil {
			err := fmt.Errorf("parameters not defined for stage %s", stage.Name)
			log.Errorf("%v", err)
			return err
		}
		pEntry := pipelineEntry{
			configStage: stage,
			stageType:   stageType,
		}
		var err error
		switch pEntry.stageType {
		case StageIngest:
			pEntry.Ingester, err = getIngester(*params)
		case StageDecode:
			pEntry.Decoder, err = getDecoder(*params)
		case StageTransform:
			pEntry.Transformer, err = getTransformer(*params)
		case StageExtract:
			pEntry.Extractor, err = getExtractor(*params)
		case StageEncode:
			pEntry.Encoder, err = getEncoder(*params)
		case StageWrite:
			pEntry.Writer, err = getWriter(*params)
		default:
			err = fmt.Errorf("invalid stage type: %v", pEntry.stageType)
		}
		if err != nil {
			return err
		}
		b.pipelineEntryMap[stage.Name] = &pEntry
		b.pipelineStages = append(b.pipelineStages, &pEntry)
		log.Debugf("pipeline = %v", b.pipelineStages)
	}
	log.Debugf("pipeline = %v", b.pipelineStages)
	return nil
}

// reads the configured Go stages and connects between them
// readStages must be invoked before this
func (b *builder) build() (*Pipeline, error) {
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
		log.Debugf("connecting stages: %s --> %s", connection.Follows, connection.Name)

		// connects source and destination node, and catches any panic from the Go-Pipes library.
		var catchErr error
		func() {
			defer func() {
				if msg := recover(); msg != nil {
					catchErr = fmt.Errorf("%q and %q stages haven't compatible input/outputs: %v",
						connection.Follows, connection.Name, msg)
				}
			}()
			src.SendsTo(dst)
		}()
		if catchErr != nil {
			return nil, catchErr
		}
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
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Encoder.Encode(i)
			}
		})
	case StageTransform:
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- transform.ExecuteTransform(pe.Transformer, i)
			}
		})
	case StageExtract:
		stage = node.AsMiddle(func(in <-chan []config.GenericMap, out chan<- []config.GenericMap) {
			for i := range in {
				out <- pe.Extractor.Extract(i)
			}
		})
	default:
		return nil, fmt.Errorf("invalid stage type: %s", pe.stageType)
	}
	b.createdStages[stageID] = stage
	return stage, nil
}

func getIngester(params config.StageParam) (ingest.Ingester, error) {
	var ingester ingest.Ingester
	var err error
	switch params.Ingest.Type {
	case "file", "file_loop", "file_chunks":
		ingester, err = ingest.NewIngestFile(params)
	case "collector":
		ingester, err = ingest.NewIngestCollector(params)
	case "kafka":
		ingester, err = ingest.NewIngestKafka(params)
	default:
		panic(fmt.Sprintf("`ingest` type %s not defined; if no encoder needed, specify `none`", params.Ingest.Type))
	}
	return ingester, err
}

func getDecoder(params config.StageParam) (decode.Decoder, error) {
	var decoder decode.Decoder
	var err error
	switch params.Decode.Type {
	case "json":
		decoder, err = decode.NewDecodeJson()
	case "aws":
		decoder, err = decode.NewDecodeAws(params)
	case "none":
		decoder, err = decode.NewDecodeNone()
	default:
		panic(fmt.Sprintf("`decode` type %s not defined; if no encoder needed, specify `none`", params.Decode.Type))
	}
	return decoder, err
}

func getWriter(params config.StageParam) (write.Writer, error) {
	var writer write.Writer
	var err error
	switch params.Write.Type {
	case "stdout":
		writer, err = write.NewWriteStdout()
	case "none":
		writer, err = write.NewWriteNone()
	case "loki":
		writer, err = write.NewWriteLoki(params)
	default:
		panic(fmt.Sprintf("`write` type %s not defined; if no encoder needed, specify `none`", params.Write.Type))
	}
	return writer, err
}

func getTransformer(params config.StageParam) (transform.Transformer, error) {
	var transformer transform.Transformer
	var err error
	switch params.Transform.Type {
	case transform.OperationGeneric:
		transformer, err = transform.NewTransformGeneric(params)
	case transform.OperationNetwork:
		transformer, err = transform.NewTransformNetwork(params)
	case transform.OperationNone:
		transformer, err = transform.NewTransformNone()
	default:
		panic(fmt.Sprintf("`transform` type %s not defined; if no encoder needed, specify `none`", params.Transform.Type))
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
		panic(fmt.Sprintf("`extract` type %s not defined; if no encoder needed, specify `none`", params.Extract.Type))
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
func findStageParameters(stage config.Stage, configParams []config.StageParam) (*config.StageParam, string) {
	log.Debugf("findStageParameters: stage = %v", stage)
	for _, param := range configParams {
		log.Debugf("findStageParameters: param = %v", param)
		if stage.Name == param.Name {
			var stageType string
			if param.Ingest.Type != "" {
				stageType = StageIngest
			}
			if param.Decode.Type != "" {
				stageType = StageDecode
			}
			if param.Transform.Type != "" {
				stageType = StageTransform
			}
			if param.Extract.Type != "" {
				stageType = StageExtract
			}
			if param.Encode.Type != "" {
				stageType = StageEncode
			}
			if param.Write.Type != "" {
				stageType = StageWrite
			}
			return &param, stageType
		}
	}
	return nil, ""
}
