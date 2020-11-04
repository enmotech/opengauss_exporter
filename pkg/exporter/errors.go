// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

type ErrorConnectToServer struct {
	Msg string
}

// Error returns error
func (e *ErrorConnectToServer) Error() string {
	return e.Msg
}
