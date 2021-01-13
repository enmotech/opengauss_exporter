// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

import (
// "database/sql"
// "fmt"
// "testing"
// "github.com/DATA-DOG/go-sqlmock"
//
)

// func Test_exporter(t *testing.T) {
// 	var (
// 		db  *sql.DB
// 		mock          sqlmock.Sqlmock
// 	)
// 	ex, err := NewExporter(WithAutoDiscovery(true),
// 		WithDNS([]string{"host=localhost user=opengauss_exporter password=mtkOP@123 port=5433 dbname=postgres sslmode=disable"}),
// 	)
// 	if err != nil {
// 		t.Error(err)
// 		return
// 	}
// 	t.Run("discoverDatabaseDSNs", func(t *testing.T) {
// 		ex.servers
// 		dnsList := ex.discoverDatabaseDSNs()
// 		fmt.Println(dnsList)
// 	})
// }
