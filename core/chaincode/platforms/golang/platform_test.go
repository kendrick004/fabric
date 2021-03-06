package golang

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"archive/tar"
	"bytes"
	"compress/gzip"
	"time"

	"github.com/spf13/viper"

	"github.com/hyperledger/fabric/core/config"
	pb "github.com/hyperledger/fabric/protos/peer"
)

func testerr(err error, succ bool) error {
	if succ && err != nil {
		return fmt.Errorf("Expected success but got %s", err)
	} else if !succ && err == nil {
		return fmt.Errorf("Expected failer but succeeded")
	}
	return nil
}

func writeBytesToPackage(name string, payload []byte, mode int64, tw *tar.Writer) error {
	//Make headers identical by using zero time
	var zeroTime time.Time
	tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(payload)), ModTime: zeroTime, AccessTime: zeroTime, ChangeTime: zeroTime, Mode: mode})
	tw.Write(payload)

	return nil
}

func generateFakeCDS(ccname, path, file string, mode int64) (*pb.ChaincodeDeploymentSpec, error) {
	codePackage := bytes.NewBuffer(nil)
	gw := gzip.NewWriter(codePackage)
	tw := tar.NewWriter(gw)

	payload := make([]byte, 25, 25)
	err := writeBytesToPackage(file, payload, mode, tw)
	if err != nil {
		return nil, err
	}

	tw.Close()
	gw.Close()

	cds := &pb.ChaincodeDeploymentSpec{
		ChaincodeSpec: &pb.ChaincodeSpec{
			ChaincodeId: &pb.ChaincodeID{
				Name: ccname,
				Path: path,
			},
		},
		CodePackage: codePackage.Bytes(),
	}

	return cds, nil
}

type spec struct {
	CCName          string
	Path, File      string
	Mode            int64
	SuccessExpected bool
	RealGen         bool
}

func TestValidateCDS(t *testing.T) {
	platform := &Platform{}

	specs := make([]spec, 0)
	specs = append(specs, spec{CCName: "NoCode", Path: "path/to/nowhere", File: "/bin/warez", Mode: 0100400, SuccessExpected: false})
	specs = append(specs, spec{CCName: "NoCode", Path: "path/to/somewhere", File: "/src/path/to/somewhere/main.go", Mode: 0100400, SuccessExpected: true})
	specs = append(specs, spec{CCName: "NoCode", Path: "path/to/somewhere", File: "/src/path/to/somewhere/warez", Mode: 0100555, SuccessExpected: false})

	for _, s := range specs {
		cds, err := generateFakeCDS(s.CCName, s.Path, s.File, s.Mode)

		err = platform.ValidateDeploymentSpec(cds)
		if s.SuccessExpected == true && err != nil {
			t.Errorf("Unexpected failure: %s", err)
		}
		if s.SuccessExpected == false && err == nil {
			t.Log("Expected validation failure")
			t.Fail()
		}
	}
}

func Test_writeGopathSrc(t *testing.T) {

	inputbuf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(inputbuf)

	err := writeGopathSrc(tw, "")
	if err != nil {
		t.Fail()
		t.Logf("Error writing gopath src: %s", err)
	}
	//ioutil.WriteFile("/tmp/chaincode_deployment.tar", inputbuf.Bytes(), 0644)

}

func Test_decodeUrl(t *testing.T) {
	cs := &pb.ChaincodeSpec{
		ChaincodeId: &pb.ChaincodeID{
			Name: "Test Chaincode",
			Path: "http://github.com/hyperledger/fabric/examples/chaincode/go/map",
		},
	}

	if _, err := decodeUrl(cs); err != nil {
		t.Fail()
		t.Logf("Error to decodeUrl unsuccessfully with valid path: %s, %s", cs.ChaincodeId.Path, err)
	}

	cs.ChaincodeId.Path = ""

	if _, err := decodeUrl(cs); err == nil {
		t.Fail()
		t.Logf("Error to decodeUrl successfully with invalid path: %s", cs.ChaincodeId.Path)
	}

	cs.ChaincodeId.Path = "/"

	if _, err := decodeUrl(cs); err == nil {
		t.Fail()
		t.Logf("Error to decodeUrl successfully with invalid path: %s", cs.ChaincodeId.Path)
	}

	cs.ChaincodeId.Path = "http:///"

	if _, err := decodeUrl(cs); err == nil {
		t.Fail()
		t.Logf("Error to decodeUrl successfully with invalid path: %s", cs.ChaincodeId.Path)
	}
}

func TestValidChaincodeSpec(t *testing.T) {
	platform := &Platform{}

	var tests = []struct {
		spec *pb.ChaincodeSpec
		succ bool
	}{
		{spec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "Test Chaincode", Path: "http://github.com/hyperledger/fabric/examples/chaincode/go/map"}}, succ: true},
		{spec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "Test Chaincode", Path: "https://github.com/hyperledger/fabric/examples/chaincode/go/map"}}, succ: true},
		{spec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "Test Chaincode", Path: "github.com/hyperledger/fabric/examples/chaincode/go/map"}}, succ: true},
		{spec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "Test Chaincode", Path: "github.com/hyperledger/fabric/bad/chaincode/go/map"}}, succ: false},
	}

	for _, tst := range tests {
		err := platform.ValidateSpec(tst.spec)
		if err = testerr(err, tst.succ); err != nil {
			t.Errorf("Error to validating chaincode spec: %s, %s", tst.spec.ChaincodeId.Path, err)
		}
	}
}

func TestGetDeploymentPayload(t *testing.T) {
	platform := &Platform{}

	var tests = []struct {
		spec *pb.ChaincodeSpec
		succ bool
	}{
		{spec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "Test Chaincode", Path: "github.com/hyperledger/fabric/examples/chaincode/go/map"}}, succ: true},
		{spec: &pb.ChaincodeSpec{ChaincodeId: &pb.ChaincodeID{Name: "Test Chaincode", Path: "github.com/hyperledger/fabric/examples/bad/go/map"}}, succ: false},
	}

	for _, tst := range tests {
		_, err := platform.GetDeploymentPayload(tst.spec)
		if err = testerr(err, tst.succ); err != nil {
			t.Errorf("Error to validating chaincode spec: %s, %s", tst.spec.ChaincodeId.Path, err)
		}
	}
}

//TestGenerateDockerBuild goes through the functions needed to do docker build
func TestGenerateDockerBuild(t *testing.T) {
	platform := &Platform{}

	specs := make([]spec, 0)
	specs = append(specs, spec{CCName: "NoCode", Path: "path/to/nowhere", File: "/bin/warez", Mode: 0100400, SuccessExpected: false})
	specs = append(specs, spec{CCName: "invalidhttp", Path: "https://not/a/valid/path", File: "/src/github.com/hyperledger/fabric/examples/chaincode/go/map/map.go", Mode: 0100400, SuccessExpected: false, RealGen: true})
	specs = append(specs, spec{CCName: "map", Path: "github.com/hyperledger/fabric/examples/chaincode/go/map", File: "/src/github.com/hyperledger/fabric/examples/chaincode/go/map/map.go", Mode: 0100400, SuccessExpected: true, RealGen: true})
	specs = append(specs, spec{CCName: "mapBadPath", Path: "github.com/hyperledger/fabric/examples/chaincode/go/map", File: "/src/github.com/hyperledger/fabric/examples/bad/path/to/map.go", Mode: 0100400, SuccessExpected: false})
	specs = append(specs, spec{CCName: "mapBadMode", Path: "github.com/hyperledger/fabric/examples/chaincode/go/map", File: "/src/github.com/hyperledger/fabric/examples/chaincode/go/map/map.go", Mode: 0100555, SuccessExpected: false})

	var err error
	for _, tst := range specs {
		inputbuf := bytes.NewBuffer(nil)
		tw := tar.NewWriter(inputbuf)

		var cds *pb.ChaincodeDeploymentSpec
		if tst.RealGen {
			cds = &pb.ChaincodeDeploymentSpec{
				ChaincodeSpec: &pb.ChaincodeSpec{
					ChaincodeId: &pb.ChaincodeID{
						Name:    tst.CCName,
						Path:    tst.Path,
						Version: "0",
					},
				},
			}
			cds.CodePackage, err = platform.GetDeploymentPayload(cds.ChaincodeSpec)
			if err = testerr(err, tst.SuccessExpected); err != nil {
				t.Errorf("test failed in GetDeploymentPayload: %s, %s", cds.ChaincodeSpec.ChaincodeId.Path, err)
			}
		} else {
			cds, err = generateFakeCDS(tst.CCName, tst.Path, tst.File, tst.Mode)
		}

		if _, err = platform.GenerateDockerfile(cds); err != nil {
			t.Errorf("could not generate docker file for a valid spec: %s, %s", cds.ChaincodeSpec.ChaincodeId.Path, err)
		}
		err = platform.GenerateDockerBuild(cds, tw)
		if err = testerr(err, tst.SuccessExpected); err != nil {
			t.Errorf("Error to validating chaincode spec: %s, %s", cds.ChaincodeSpec.ChaincodeId.Path, err)
		}
	}
}

func TestMain(m *testing.M) {
	viper.SetConfigName("core")
	viper.SetEnvPrefix("CORE")
	config.AddDevConfigPath(nil)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("could not read config %s\n", err)
		os.Exit(-1)
	}
	os.Exit(m.Run())
}
