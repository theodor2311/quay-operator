package controller

import (
	"github.com/theodor2311/quay-operator/pkg/controller/quayecosystem"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, quayecosystem.Add)
}
