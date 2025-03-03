package main

import (
	"context"

	"github.com/k11v/merch/api/merch"
)

// GetAPIHealth implements merch.StrictServerInterface.
func (h *Handler) GetAPIHealth(ctx context.Context, request merch.GetAPIHealthRequestObject) (merch.GetAPIHealthResponseObject, error) {
	status := "ok"
	return merch.GetAPIHealth200JSONResponse{Status: &status}, nil
}
