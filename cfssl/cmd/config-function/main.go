package main

import (
	"fmt"
	"os"

	"github.com/bzub/config-functions/cfssl"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
)

func main() {
	rw := &kio.ByteReadWriter{
		Reader:                os.Stdin,
		Writer:                os.Stdout,
		KeepReaderAnnotations: true,
	}

	cfsslFilter := &cfssl.ConfigFunction{}
	cfsslFilter.RW = rw

	err := kio.Pipeline{
		Inputs: []kio.Reader{rw},
		Filters: []kio.Filter{
			cfsslFilter,
			&filters.MergeFilter{},
			&filters.FormatFilter{},
		},
		Outputs: []kio.Writer{rw},
	}.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
