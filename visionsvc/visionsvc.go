package visionsvc

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"sync"

	"github.com/openalpr/openalpr/src/bindings/go/openalpr"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	vis "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
)

var errUnimplemented = errors.New("unimplemented")
var Model = resource.NewModel("viam-soleng", "vision", "openalpr")
var PrettyName = "Viam openalpr vision service"
var Description = "A module of the Viam vision service that crops an image to an initial detection bounding box and then processes the cropped image with the provided vision service"

type Config struct {
	Country    string `json:"country"`
	ConfigFile string `json:"config_file"`
	RuntimeDir string `json:"runtime_dir"`
}

type myVisionSvc struct {
	resource.Named
	logger     logging.Logger
	country    string
	configFile string
	runtimeDir string
	mu         sync.RWMutex
	cancelCtx  context.Context
	cancelFunc func()
	done       chan bool

	alpr openalpr.Alpr
}

func init() {
	resource.RegisterService(
		vision.API,
		Model,
		resource.Registration[vision.Service, *Config]{
			Constructor: newService,
		})
}

func newService(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (vision.Service, error) {
	logger.Debugf("Starting %s %s", PrettyName)
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	svc := myVisionSvc{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		mu:         sync.RWMutex{},
		done:       make(chan bool),
	}

	if err := svc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return &svc, nil
}

func (cfg *Config) Validate(path string) ([]string, error) {
	return []string{}, nil
}

// Reconfigure reconfigures with new settings.
func (svc *myVisionSvc) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.logger.Debugf("Reconfiguring %s", PrettyName)
	// TODO: Make NewAlpr configurable
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	if newConf.Country != "" {
		svc.country = newConf.Country
	} else {
		svc.country = "us"
	}
	if newConf.ConfigFile != "" {
		svc.configFile = newConf.ConfigFile
	} else {
		svc.configFile = ""
	}
	if newConf.RuntimeDir != "" {
		svc.runtimeDir = newConf.RuntimeDir
	} else {
		svc.runtimeDir = "$APPDIR/usr/share/runtime_data"
	}
	svc.alpr = *openalpr.NewAlpr(svc.country, svc.configFile, svc.runtimeDir) // Defaults ("us", "", "./runtime_data")
	if !svc.alpr.IsLoaded() {
		return errors.New("openalpr failed to load")
	}
	svc.alpr.SetTopN(20)
	svc.logger.Debugf("openalpr version: %v", openalpr.GetVersion())
	svc.logger.Debug("**** Reconfigured ****")
	return nil
}

// Classifications can be implemented to extend functionality but returns unimplemented currently.
func (svc *myVisionSvc) Classifications(ctx context.Context, img image.Image, n int, extra map[string]interface{}) (classification.Classifications, error) {
	return nil, errUnimplemented
}

// ClassificationsFromCamera can be implemented to extend functionality but returns unimplemented currently.
func (svc *myVisionSvc) ClassificationsFromCamera(ctx context.Context, cameraName string, n int, extra map[string]interface{}) (classification.Classifications, error) {
	return nil, errUnimplemented
}

func (svc *myVisionSvc) Detections(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
	detections, err := svc.detectAlpr(img)
	if err != nil {
		return nil, err
	}
	return detections, nil
}

func (svc *myVisionSvc) DetectionsFromCamera(ctx context.Context, camera string, extra map[string]interface{}) ([]objectdetection.Detection, error) {
	svc.detectAlpr(nil)
	return nil, nil
}

// ObjectPointClouds can be implemented to extend functionality but returns unimplemented currently.
func (s *myVisionSvc) GetObjectPointClouds(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*vis.Object, error) {
	return nil, errUnimplemented
}

// DoCommand can be implemented to extend functionality but returns unimplemented currently.
func (s *myVisionSvc) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errUnimplemented
}

// The close method is executed when the component is shut down
func (svc *myVisionSvc) Close(ctx context.Context) error {
	svc.logger.Debugf("Shutting down %s", PrettyName)
	svc.alpr.Unload()
	return errUnimplemented
}

func (svc *myVisionSvc) detectAlpr(img image.Image) ([]objectdetection.Detection, error) {
	/*
		resultFromFilePath, err := svc.alpr.RecognizeByFilePath("lp.jpg")
		if err != nil {
			fmt.Println(err)
		}
		svc.logger.Infof("Detections: %v", resultFromFilePath)
		//fmt.Printf("%+v\n", resultFromFilePath)
		//fmt.Printf("\n\n\n")
	*/
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, nil)
	if err != nil {
		return nil, err
	}
	imageBytes := buf.Bytes()

	/*
		imageBytes, err := os.ReadFile("lp.jpg")
		if err != nil {
			fmt.Println(err)
		}
	*/
	resultFromBlob, err := svc.alpr.RecognizeByBlob(imageBytes)
	if err != nil {
		return nil, err
	}
	svc.logger.Debugf("%v", resultFromBlob)
	detections := []objectdetection.Detection{}
	for _, result := range resultFromBlob.Plates {
		minPoint := image.Point{result.PlatePoints[0].X, result.PlatePoints[0].Y}
		maxPoint := image.Point{result.PlatePoints[3].X, result.PlatePoints[3].Y}
		bbox := image.Rectangle{minPoint, maxPoint}
		detection := objectdetection.NewDetection(bbox, float64(result.TopNPlates[result.PlateIndex].OverallConfidence), result.BestPlate)
		detections = append(detections, detection)
	}
	return detections, nil
}
