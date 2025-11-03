package monitor

import (
	"database/sql"
	"encoding/json"
)

// executeApplicationOp 执行应用相关的数据库操作
func (m *Monitor) executeApplicationOp(tx *sql.Tx, op dbOperation) error {
	data := op.data.(map[string]interface{})

	switch op.opType {
	case "insert":
		metadataJSON, _ := json.Marshal(data["metadata"])
		_, err := tx.Exec(
			`INSERT INTO applications (app_id, metadata, status, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			data["app_id"], string(metadataJSON), data["status"], data["created_at"], data["updated_at"])
		return err

	case "update":
		query := `UPDATE applications SET status = ?, updated_at = ?`
		args := []interface{}{data["status"], data["updated_at"]}

		if startTime, ok := data["start_time"].(*int64); ok && startTime != nil {
			query += `, start_time = COALESCE(start_time, ?)`
			args = append(args, *startTime)
		}
		if endTime, ok := data["end_time"].(*int64); ok && endTime != nil {
			query += `, end_time = ?`
			args = append(args, *endTime)
		}
		if dag, ok := data["dag"]; ok && dag != nil {
			dagJSON, _ := json.Marshal(dag)
			query += `, dag = ?`
			args = append(args, string(dagJSON))
		}

		query += ` WHERE app_id = ?`
		args = append(args, data["app_id"])

		_, err := tx.Exec(query, args...)
		return err

	case "delete":
		_, err := tx.Exec(`DELETE FROM applications WHERE app_id = ?`, data["app_id"])
		return err
	}

	return nil
}

// executeNodeStateOp 执行节点状态相关的数据库操作
func (m *Monitor) executeNodeStateOp(tx *sql.Tx, op dbOperation) error {
	data := op.data.(map[string]interface{})

	switch op.opType {
	case "insert", "upsert":
		var resultJSON sql.NullString
		if result, ok := data["result"]; ok && result != nil {
			if jsonData, err := json.Marshal(result); err == nil {
				resultJSON = sql.NullString{String: string(jsonData), Valid: true}
			}
		}

		_, err := tx.Exec(
			`INSERT INTO node_states (app_id, node_id, type, status, start_time, end_time, duration, retry_count, error_message, result, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(app_id, node_id) DO UPDATE SET
			   type = excluded.type,
			   status = excluded.status,
			   start_time = COALESCE(node_states.start_time, excluded.start_time),
			   end_time = excluded.end_time,
			   duration = excluded.duration,
			   retry_count = excluded.retry_count,
			   error_message = excluded.error_message,
			   result = excluded.result,
			   updated_at = excluded.updated_at`,
			data["app_id"], data["node_id"], data["type"], data["status"],
			data["start_time"], data["end_time"], data["duration"],
			data["retry_count"], data["error_message"], resultJSON, data["updated_at"])
		return err
	}

	return nil
}

// executeTaskOp 执行任务相关的数据库操作
func (m *Monitor) executeTaskOp(tx *sql.Tx, op dbOperation) error {
	data := op.data.(map[string]interface{})

	switch op.opType {
	case "insert":
		paramsJSON, _ := json.Marshal(data["parameters"])
		_, err := tx.Exec(
			`INSERT INTO tasks (task_id, app_id, node_id, status, function_name, parameters, start_time, worker_id, retry_count, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			data["task_id"], data["app_id"], data["node_id"], data["status"], data["function_name"],
			string(paramsJSON), data["start_time"], data["worker_id"], data["retry_count"],
			data["created_at"], data["updated_at"])
		return err

	case "update":
		_, err := tx.Exec(
			`UPDATE tasks SET status = ?, end_time = ?, error_message = ?, updated_at = ?,
			   duration = ? - COALESCE(start_time, ?)
			 WHERE task_id = ?`,
			data["status"], data["end_time"], data["error_message"], data["updated_at"],
			data["end_time"], data["end_time"], data["task_id"])
		return err
	}

	return nil
}

// executeMetricOp 执行指标相关的数据库操作
func (m *Monitor) executeMetricOp(tx *sql.Tx, op dbOperation) error {
	data := op.data.(map[string]interface{})

	if op.opType == "insert" {
		labelsJSON, _ := json.Marshal(data["labels"])
		_, err := tx.Exec(
			`INSERT INTO metrics (name, type, value, labels, unit, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			data["name"], data["type"], data["value"], string(labelsJSON), data["unit"], data["timestamp"])
		return err
	}

	return nil
}

// executeEventOp 执行事件相关的数据库操作
func (m *Monitor) executeEventOp(tx *sql.Tx, op dbOperation) error {
	data := op.data.(map[string]interface{})

	if op.opType == "insert" {
		detailsJSON, _ := json.Marshal(data["details"])
		_, err := tx.Exec(
			`INSERT INTO events (id, type, level, app_id, node_id, task_id, worker_id, message, details, source, timestamp)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			data["id"], data["type"], data["level"], data["app_id"], data["node_id"], data["task_id"],
			data["worker_id"], data["message"], string(detailsJSON), data["source"], data["timestamp"])
		return err
	}

	return nil
}
