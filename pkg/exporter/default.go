// Copyright © 2020 Bin Liu <bin.liu@enmotech.com>

package exporter

// var (
// 	ogVersionName = "OG_VERSION"
// )

var (
	pgLock = &QueryInstance{
		Name: "pg_lock",
		Desc: "OpenGauss lock distribution by mode",
		Queries: []*Query{
			{
				Version: ">=0.0.0",
				SQL: `SELECT
  datname, mode, coalesce(count, 0) AS count
FROM (
  SELECT d.oid AS database, d.datname, l.mode FROM pg_database d,
           unnest(ARRAY ['AccessShareLock','RowShareLock','RowExclusiveLock','ShareUpdateExclusiveLock',
               'ShareLock','ShareRowExclusiveLock','ExclusiveLock','AccessExclusiveLock']
               ) l(mode)
WHERE d.datname NOT IN ('template0','template1')) base
LEFT JOIN (SELECT database, mode, count(1) AS count FROM pg_locks
WHERE database IS NOT NULL GROUP BY database, mode) cnt USING (database, mode);`,
			},
		},
		Metrics: []*Column{
			{Name: "datname", Desc: "Name of this database", Usage: LABEL},
			{Name: "mode", Desc: "Type of Lock", Usage: LABEL},
			{Name: "count", Desc: "Number of locks", Usage: GAUGE},
		},
		Public: true,
	}
	pgStatReplication = &QueryInstance{
		Name: "pg_stat_replication",
		Desc: "",
		Queries: []*Query{
			{
				Name: "pg_stat_replication",
				SQL: `SELECT *,
  (case pg_is_in_recovery() when 't' then null else pg_current_xlog_location() end) AS pg_current_xlog_location,
  (case pg_is_in_recovery() when 't' then null else pg_xlog_location_diff(pg_current_xlog_location(), receiver_replay_location)::float end) AS pg_xlog_location_diff
FROM pg_stat_replication`,
				Version: ">=1.0.0",
			},
		},
		Metrics: []*Column{
			{Name: "procpid", Usage: DISCARD, Desc: "Process ID of a WAL sender process"},
			{Name: "pid", Usage: DISCARD, Desc: "Process ID of a WAL sender process"},
			{Name: "usesysid", Usage: DISCARD, Desc: "OID of the user logged into this WAL sender process"},
			{Name: "usename", Usage: DISCARD, Desc: "Name of the user logged into this WAL sender process"},
			{Name: "application_name", Usage: LABEL, Desc: "Name of the application that is connected to this WAL sender"},
			{Name: "client_addr", Usage: LABEL, Desc: "IP address of the client connected to this WAL sender. If this field is null, it indicates that the client is connected via a Unix socket on the server machine."},
			{Name: "client_hostname", Usage: DISCARD, Desc: "Host name of the connected client, as reported by a reverse DNS lookup of client_addr. This field will only be non-null for IP connections, and only when log_hostname is enabled."},
			{Name: "client_port", Usage: DISCARD, Desc: "TCP port number that the client is using for communication with this WAL sender, or -1 if a Unix socket is used"},
			{Name: "backend_start", Usage: DISCARD, Desc: "with time zone      Time when this process was started, i.e., when the client connected to this WAL sender"},
			{Name: "backend_xmin", Usage: DISCARD, Desc: "The current backend's xmin horizon."},
			{Name: "state", Usage: LABEL, Desc: "Current WAL sender state"},
			{Name: "sender_sent_location", Usage: DISCARD, Desc: "Last transaction log position sent on this connection"},
			{Name: "receiver_write_location", Usage: DISCARD, Desc: "Last transaction log position written to disk by this standby server"},
			{Name: "receiver_flush_location", Usage: DISCARD, Desc: "Last transaction log position flushed to disk by this standby server"},
			{Name: "receiver_replay_location", Usage: DISCARD, Desc: "Last transaction log position replayed into the database on this standby server"},
			{Name: "sync_priority", Usage: DISCARD, Desc: "Priority of this standby server for being chosen as the synchronous standby"},
			{Name: "sync_state", Usage: DISCARD, Desc: "Synchronous state of this standby server"},
			{Name: "pg_current_xlog_location", Usage: DISCARD, Desc: "pg_current_xlog_location"},
			{Name: "pg_xlog_location_diff", Usage: GAUGE, Desc: "Lag in bytes between primary and slave"},
			{Name: "sent_location", Usage: DISCARD, Desc: "Last transaction log position sent on this connection"},
			{Name: "write_location", Usage: DISCARD, Desc: "Last transaction log position written to disk by this standby server"},
			{Name: "flush_location", Usage: DISCARD, Desc: "Last transaction log position flushed to disk by this standby server"},
			{Name: "replay_location", Usage: DISCARD, Desc: "Last transaction log position replayed into the database on this standby server"},
			{Name: "sent_lsn", Usage: DISCARD, Desc: "Last transaction log position sent on this connection"},
			{Name: "write_lsn", Usage: DISCARD, Desc: "Last transaction log position written to disk by this standby server"},
			{Name: "flush_lsn", Usage: DISCARD, Desc: "Last transaction log position flushed to disk by this standby server"},
			{Name: "replay_lsn", Usage: DISCARD, Desc: "Last transaction log position replayed into the database on this standby server"},
			{Name: "sync_priority", Usage: DISCARD, Desc: "Priority of this standby server for being chosen as the synchronous standby"},
			{Name: "sync_state", Usage: DISCARD, Desc: "Synchronous state of this standby server"},
			{Name: "slot_name", Usage: LABEL, Desc: "A unique, cluster-wide identifier for the replication slot"},
			{Name: "plugin", Usage: DISCARD, Desc: "The base name of the shared object containing the output plugin this logical slot is using, or null for physical slots"},
			{Name: "slot_type", Usage: DISCARD, Desc: "The slot type - physical or logical"},
			{Name: "datoid", Usage: DISCARD, Desc: "The OID of the database this slot is associated with, or null. Only logical slots have an associated database"},
			{Name: "database", Usage: DISCARD, Desc: "The name of the database this slot is associated with, or null. Only logical slots have an associated database"},
			{Name: "active", Usage: DISCARD, Desc: "True if this slot is currently actively being used"},
			{Name: "active_pid", Usage: DISCARD, Desc: "Process ID of a WAL sender process"},
			{Name: "xmin", Usage: DISCARD, Desc: "The oldest transaction that this slot needs the database to retain. VACUUM cannot remove tuples deleted by any later transaction"},
			{Name: "catalog_xmin", Usage: DISCARD, Desc: "The oldest transaction affecting the system catalogs that this slot needs the database to retain. VACUUM cannot remove catalog tuples deleted by any later transaction"},
			{Name: "restart_lsn", Usage: DISCARD, Desc: "The address (LSN) of oldest WAL which still might be required by the consumer of this slot and thus won't be automatically removed during checkpoints"},
			{Name: "pg_current_xlog_location", Usage: DISCARD, Desc: "pg_current_xlog_location"},
			{Name: "pg_current_wal_lsn", Usage: DISCARD, Desc: "pg_current_xlog_location"},
			{Name: "pg_current_wal_lsn_bytes", Usage: GAUGE, Desc: "WAL position in bytes"},
			{Name: "pg_xlog_location_diff", Usage: GAUGE, Desc: "Lag in bytes between primary and slave"},
			{Name: "pg_wal_lsn_diff", Usage: GAUGE, Desc: "Lag in bytes between primary and slave"},
			{Name: "confirmed_flush_lsn", Usage: DISCARD, Desc: "LSN position a consumer of a slot has confirmed flushing the data received"},
			{Name: "write_lag", Usage: DISCARD, Desc: "Time elapsed between flushing recent WAL locally and receiving notification that this standby server has written it (but not yet flushed it or applied it). This can be used to gauge the delay that synchronous_commit level remote_write incurred while committing if this server was configured as a synchronous standby."},
			{Name: "flush_lag", Usage: DISCARD, Desc: "Time elapsed between flushing recent WAL locally and receiving notification that this standby server has written and flushed it (but not yet applied it). This can be used to gauge the delay that synchronous_commit level remote_flush incurred while committing if this server was configured as a synchronous standby."},
			{Name: "replay_lag", Usage: DISCARD, Desc: "Time elapsed between flushing recent WAL locally and receiving notification that this standby server has written, flushed and applied it. This can be used to gauge the delay that synchronous_commit level remote_apply incurred while committing if this server was configured as a synchronous standby."},
		},
		Public: true,
	}
	pgStatActivity = &QueryInstance{
		Name: "pg_stat_activity",
		Desc: "OpenGauss backend activity group by state",
		Queries: []*Query{
			{
				SQL: `SELECT datname,
       state,
       coalesce(count, 0)             AS count,
       coalesce(max_duration, 0)      AS max_duration,
       coalesce(max_tx_duration, 0)   AS max_tx_duration,
       coalesce(max_conn_duration, 0) AS max_conn_duration
FROM (SELECT d.oid AS database, d.datname, a.state
      FROM pg_database d,
           unnest(ARRAY ['active','idle','idle in transaction','idle in transaction (aborted)','fastpath function call','disabled']) a(state)
      WHERE d.datname NOT IN ('template0','template1')) base
         LEFT JOIN (
    SELECT datname, state,
           count(*) AS count,
           max(extract(epoch from now() - state_change)) AS max_duration,
           max(extract(epoch from now() - xact_start))   AS max_tx_duration,
           max(extract(epoch from now() - backend_start)) AS max_conn_duration
    FROM pg_stat_activity WHERE pid <> pg_backend_pid()
    GROUP BY datname, state
) a USING (datname, state);`,
				Version: ">=1.0.0",
			},
		},
		Metrics: []*Column{
			{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
			{Name: "state", Usage: LABEL, Desc: "connection state"},
			{Name: "count", Usage: GAUGE, Desc: "number of connections in this state"},
			{Name: "max_duration", Usage: GAUGE, Desc: "max duration since state change among (datname, state)"},
			{Name: "max_tx_duration", Usage: GAUGE, Desc: "max duration in seconds any active transaction has been running"},
			{Name: "max_conn_duration", Usage: GAUGE, Desc: "max backend session duration since state change among (datname, state)"},
		},
		Public: true,
	}
	pgDatabase = &QueryInstance{
		Name: "pg_database",
		Desc: "OpenGauss Database size",
		Queries: []*Query{
			{
				SQL:     `SELECT pg_database.datname, pg_database_size(pg_database.datname) as size_bytes FROM pg_database where datname NOT IN ('template0','template1')`,
				Version: ">=0.0.0",
			},
		},
		Metrics: []*Column{
			{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
			{Name: "size_bytes", Usage: GAUGE, Desc: "Disk space used by the database"},
		},
		Public: true,
	}
	pgStatBgWriter = &QueryInstance{
		Name: "pg_stat_bgwriter",
		Desc: "OpenGauss background writer metrics",
		Queries: []*Query{
			{
				SQL: `SELECT checkpoints_timed,
    checkpoints_req,
    checkpoint_write_time,
    checkpoint_sync_time,
    buffers_checkpoint,
    buffers_clean,
    buffers_backend,
    maxwritten_clean,
    buffers_backend_fsync,
    buffers_alloc,
    stats_reset
FROM pg_stat_bgwriter`,
				Version: ">=0.0.0",
			},
		},
		Metrics: []*Column{
			{Name: "checkpoints_timed", Usage: COUNTER, Desc: "scheduled checkpoints that have been performed"},
			{Name: "checkpoints_req", Usage: COUNTER, Desc: "requested checkpoints that have been performed"},
			{Name: "checkpoint_write_time", Usage: COUNTER, Desc: "time spending on writing files to disk, in µs"},
			{Name: "checkpoint_sync_time", Usage: COUNTER, Desc: "time spending on syncing files to disk, in µs"},
			{Name: "buffers_checkpoint", Usage: COUNTER, Desc: "buffers written during checkpoints"},
			{Name: "buffers_clean", Usage: COUNTER, Desc: "buffers written by the background writer"},
			{Name: "buffers_backend", Usage: COUNTER, Desc: "buffers written directly by a backend"},
			{Name: "maxwritten_clean", Usage: COUNTER, Desc: "times that bgwriter stopped a cleaning scan"},
			{Name: "buffers_backend_fsync", Usage: COUNTER, Desc: "times a backend had to execute its own fsync"},
			{Name: "buffers_alloc", Usage: COUNTER, Desc: "buffers allocated"},
			{Name: "stats_reset", Usage: COUNTER, Desc: "time when statistics were last reset"},
		},
		Public: true,
	}
	pgStatDatabase = &QueryInstance{
		Name: "pg_stat_database",
		Desc: "OpenGauss database statistics",
		Queries: []*Query{
			{
				SQL:     "select * from pg_stat_database where datname NOT IN ('template0','template1')",
				Version: ">=0.0.0",
			},
		},
		Metrics: []*Column{
			{Name: "datid", Usage: LABEL, Desc: "OID of a database"},
			{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
			{Name: "numbackends", Usage: GAUGE, Desc: "Number of backends currently connected to this database. This is the only column in this view that returns a value reflecting current state; all other columns return the accumulated values since the last reset."},
			{Name: "xact_commit", Usage: COUNTER, Desc: "Number of transactions in this database that have been committed"},
			{Name: "xact_rollback", Usage: COUNTER, Desc: "Number of transactions in this database that have been rolled back"},
			{Name: "blks_read", Usage: COUNTER, Desc: "Number of disk blocks read in this database"},
			{Name: "blks_hit", Usage: COUNTER, Desc: "Number of times disk blocks were found already in the buffer cache, so that a read was not necessary (this only includes hits in the OpenGauss buffer cache, not the operating system's file system cache)"},
			{Name: "tup_returned", Usage: COUNTER, Desc: "Number of rows returned by queries in this database"},
			{Name: "tup_fetched", Usage: COUNTER, Desc: "Number of rows fetched by queries in this database"},
			{Name: "tup_inserted", Usage: COUNTER, Desc: "Number of rows inserted by queries in this database"},
			{Name: "tup_updated", Usage: COUNTER, Desc: "Number of rows updated by queries in this database"},
			{Name: "tup_deleted", Usage: COUNTER, Desc: "Number of rows deleted by queries in this database"},
			{Name: "conflicts", Usage: COUNTER, Desc: "Number of queries canceled due to conflicts with recovery in this database. (Conflicts occur only on standby servers; see pg_stat_database_conflicts for details.)"},
			{Name: "temp_files", Usage: COUNTER, Desc: "Number of temporary files created by queries in this database. All temporary files are counted, regardless of why the temporary file was created (e.g., sorting or hashing), and regardless of the log_temp_files setting."},
			{Name: "temp_bytes", Usage: COUNTER, Desc: "Total amount of data written to temporary files by queries in this database. All temporary files are counted, regardless of why the temporary file was created, and regardless of the log_temp_files setting."},
			{Name: "deadlocks", Usage: COUNTER, Desc: "Number of deadlocks detected in this database"},
			{Name: "blk_read_time", Usage: COUNTER, Desc: "Time spent reading data file blocks by backends in this database, in milliseconds"},
			{Name: "blk_write_time", Usage: COUNTER, Desc: "Time spent writing data file blocks by backends in this database, in milliseconds"},
			{Name: "stats_reset", Usage: COUNTER, Desc: "Time at which these statistics were last reset"},
		},
		Public: true,
	}
	pgStatDatabaseConflicts = &QueryInstance{
		Name: "pg_stat_database_conflicts",
		Desc: "OpenGauss database statistics conflicts",
		Queries: []*Query{
			{
				SQL:     "select * from pg_stat_database_conflicts where datname NOT IN ('template0','template1')",
				Version: ">=0.0.0",
			},
		},
		Metrics: []*Column{
			{Name: "datid", Usage: LABEL, Desc: "OID of a database"},
			{Name: "datname", Usage: LABEL, Desc: "Name of this database"},
			{Name: "confl_tablespace", Usage: COUNTER, Desc: "Number of queries in this database that have been canceled due to dropped tablespaces"},
			{Name: "confl_lock", Usage: COUNTER, Desc: "Number of queries in this database that have been canceled due to lock timeouts"},
			{Name: "confl_snapshot", Usage: COUNTER, Desc: "Number of queries in this database that have been canceled due to old snapshots"},
			{Name: "confl_bufferpin", Usage: COUNTER, Desc: "Number of queries in this database that have been canceled due to pinned buffers"},
			{Name: "confl_deadlock", Usage: COUNTER, Desc: "Number of queries in this database that have been canceled due to deadlocks"},
		},
	}
)

var (
	defaultMonList = map[string]*QueryInstance{
		"pg_lock":                    pgLock,
		"pg_stat_replication":        pgStatReplication,
		"pg_stat_activity":           pgStatActivity,
		"pg_database":                pgDatabase,
		"pg_stat_bgwriter":           pgStatBgWriter,
		"pg_stat_database":           pgStatDatabase,
		"pg_stat_database_conflicts": pgStatDatabaseConflicts,
	}
)
