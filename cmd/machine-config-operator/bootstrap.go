package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/openshift/machine-config-operator/pkg/operator"
	"github.com/openshift/machine-config-operator/pkg/version"
)

var (
	bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Machine Config Operator in bootstrap mode",
		Long:  "",
		Run:   runBootstrapCmd,
	}

	bootstrapOpts struct {
		configFile          string
		imagesConfigMapFile string
		mccImage            string
		mcsImage            string
		mcdImage            string
		destinationDir      string
	}
)

func init() {
	rootCmd.AddCommand(bootstrapCmd)
	bootstrapCmd.PersistentFlags().StringVar(&bootstrapOpts.destinationDir, "dest-dir", "", "The destination directory where MCO writes the manifests.")
	bootstrapCmd.PersistentFlags().StringVar(&bootstrapOpts.imagesConfigMapFile, "images-json-configmap", "", "ConfigMap that contains images.json for MCO.")
	bootstrapCmd.PersistentFlags().StringVar(&bootstrapOpts.mccImage, "machine-config-controller-image", "", "Image for Machine Config Controller. (this cannot be set if --images-json-configmap is set)")
	bootstrapCmd.PersistentFlags().StringVar(&bootstrapOpts.mcsImage, "machine-config-server-image", "", "Image for Machine Config Server. (this cannot be set if --images-json-configmap is set)")
	bootstrapCmd.PersistentFlags().StringVar(&bootstrapOpts.mcdImage, "machine-config-daemon-image", "", "Image for Machine Config Daemon. (this cannot be set if --images-json-configmap is set)")
	bootstrapCmd.PersistentFlags().StringVar(&bootstrapOpts.configFile, "config-file", "", "ClusterConfig ConfigMap file.")
}

func runBootstrapCmd(cmd *cobra.Command, args []string) {
	flag.Set("logtostderr", "true")
	flag.Parse()

	// To help debugging, immediately log version
	glog.Infof("Version: %+v", version.Version)

	if bootstrapOpts.destinationDir == "" {
		glog.Fatal("--dest-dir cannot be empty")
	}

	if bootstrapOpts.configFile == "" {
		glog.Fatal("--config-file cannot be empty")
	}

	if bootstrapOpts.imagesConfigMapFile != "" &&
		(bootstrapOpts.mccImage != "" ||
			bootstrapOpts.mcsImage != "" ||
			bootstrapOpts.mcdImage != "") {
		glog.Fatal("both --images-json-configmap and --machine-config-{controller,server,daemon}-image flags cannot be set")
	}

	imgs := operator.DefaultImages()
	if bootstrapOpts.imagesConfigMapFile != "" {
		imgsRaw, err := rawImagesFromConfigMapOnDisk(bootstrapOpts.imagesConfigMapFile)
		if err != nil {
			glog.Fatal(err)
		}
		if err := json.Unmarshal([]byte(imgsRaw), &imgs); err != nil {
			glog.Fatal(err)
		}
	} else {
		imgs.MachineConfigController = bootstrapOpts.mccImage
		imgs.MachineConfigServer = bootstrapOpts.mcsImage
		imgs.MachineConfigDaemon = bootstrapOpts.mcdImage
	}
	if err := operator.RenderBootstrap(
		bootstrapOpts.configFile,
		rootOpts.etcdCAFile, rootOpts.rootCAFile,
		imgs,
		bootstrapOpts.destinationDir,
	); err != nil {
		glog.Fatalf("error rendering bootstrap manifests: %v", err)
	}
}

func rawImagesFromConfigMapOnDisk(file string) ([]byte, error) {
	data, err := ioutil.ReadFile(bootstrapOpts.imagesConfigMapFile)
	if err != nil {
		return nil, err
	}
	obji, err := runtime.Decode(scheme.Codecs.UniversalDecoder(corev1.SchemeGroupVersion), data)
	if err != nil {
		return nil, err
	}
	cm, ok := obji.(*corev1.ConfigMap)
	if !ok {
		return nil, fmt.Errorf("expected *corev1.ConfigMap found %T", obji)
	}
	return []byte(cm.Data["images.json"]), nil
}
