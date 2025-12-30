// 索引性能压测脚本
// 用法: go run scripts/benchmark_indexes.go
package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	totalRecords    = 10000
	concurrentCount = 10
)

var statuses = []string{"001", "002", "100", "200", "201", "299", "300", "301", "399", "400"}
var stepNames = []string{"获取元数据", "下载视频", "下载字幕", "下载封面", "翻译字幕", "AI增强元数据", "确认元数据", "上传到Bilibili", "上传字幕到Bilibili"}

func main() {
	fmt.Println("========================================")
	fmt.Println("🚀 索引性能压测脚本")
	fmt.Println("========================================")

	// 连接数据库
	db, err := sql.Open("sqlite3", "./bili_up.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// 1. 插入测试数据
	fmt.Printf("\n📝 插入 %d 条测试数据...\n", totalRecords)
	insertTestData(db)

	// 2. 统计数据
	showStats(db)

	// 3. 运行查询性能测试
	fmt.Println("\n========================================")
	fmt.Println("⚡ 查询性能测试")
	fmt.Println("========================================")

	// 测试 idx_user_status
	benchmarkQuery(db, "idx_user_status",
		"SELECT * FROM cw_saved_videos WHERE user_id = 1 AND status = '200'")

	// 测试 idx_status_created
	benchmarkQuery(db, "idx_status_created",
		"SELECT * FROM cw_saved_videos WHERE status = '200' ORDER BY created_at ASC LIMIT 10")

	// 测试 idx_status_processing (延迟上传查询)
	benchmarkQuery(db, "idx_status_processing",
		"SELECT * FROM cw_saved_videos WHERE status = '200' AND processing_completed_at IS NOT NULL ORDER BY processing_completed_at ASC LIMIT 1")

	// 测试 idx_status_subtitle (字幕上传查询)
	benchmarkQuery(db, "idx_status_subtitle",
		"SELECT * FROM cw_saved_videos WHERE status = '300' AND subtitle_scheduled_at <= datetime('now') ORDER BY subtitle_scheduled_at ASC LIMIT 1")

	// 测试 idx_video_step
	benchmarkQuery(db, "idx_video_step",
		"SELECT * FROM cw_task_steps WHERE video_id = 'test_video_5000' AND step_name = '下载视频'")

	// 4. EXPLAIN 分析
	fmt.Println("\n========================================")
	fmt.Println("📊 EXPLAIN 查询计划分析")
	fmt.Println("========================================")

	explainQuery(db, "用户+状态查询",
		"EXPLAIN QUERY PLAN SELECT * FROM cw_saved_videos WHERE user_id = 1 AND status = '200'")

	explainQuery(db, "状态+时间排序",
		"EXPLAIN QUERY PLAN SELECT * FROM cw_saved_videos WHERE status = '200' ORDER BY created_at ASC LIMIT 1")

	explainQuery(db, "视频+步骤查询",
		"EXPLAIN QUERY PLAN SELECT * FROM cw_task_steps WHERE video_id = 'test_video_1' AND step_name = '下载视频'")

	fmt.Println("\n========================================")
	fmt.Println("✅ 压测完成!")
	fmt.Println("========================================")

	// 5. 清理测试数据
	fmt.Println("\n🧹 清理测试数据...")
	cleanupTestData(db)
	fmt.Println("✅ 测试数据已清理")
}

func insertTestData(db *sql.DB) {
	start := time.Now()

	var wg sync.WaitGroup
	recordsPerWorker := totalRecords / concurrentCount

	for w := 0; w < concurrentCount; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			tx, _ := db.Begin()
			defer tx.Rollback()

			startIdx := workerID * recordsPerWorker
			for i := 0; i < recordsPerWorker; i++ {
				idx := startIdx + i
				videoID := fmt.Sprintf("test_video_%d", idx)
				userID := rand.Intn(100) + 1
				status := statuses[rand.Intn(len(statuses))]
				now := time.Now().Add(-time.Duration(rand.Intn(24*30)) * time.Hour)

				// 插入 cw_saved_videos
				_, err := tx.Exec(`
					INSERT OR IGNORE INTO cw_saved_videos 
					(video_id, url, title, status, user_id, created_at, updated_at, processing_completed_at, subtitle_scheduled_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					videoID,
					"https://youtube.com/watch?v="+videoID,
					"Test Video "+videoID,
					status,
					userID,
					now,
					now,
					now.Add(time.Hour),
					now.Add(2*time.Hour),
				)
				if err != nil {
					continue
				}

				// 插入 cw_task_steps
				for _, stepName := range stepNames {
					tx.Exec(`
						INSERT OR IGNORE INTO cw_task_steps 
						(video_id, step_name, step_order, status, created_at, updated_at)
						VALUES (?, ?, ?, ?, ?, ?)`,
						videoID,
						stepName,
						1,
						"pending",
						now,
						now,
					)
				}
			}
			tx.Commit()
		}(w)
	}

	wg.Wait()
	fmt.Printf("✅ 插入完成，耗时: %v\n", time.Since(start))
}

func showStats(db *sql.DB) {
	var videoCount, stepCount int
	db.QueryRow("SELECT COUNT(*) FROM cw_saved_videos").Scan(&videoCount)
	db.QueryRow("SELECT COUNT(*) FROM cw_task_steps").Scan(&stepCount)

	fmt.Printf("\n📊 数据统计:\n")
	fmt.Printf("   - cw_saved_videos: %d 条\n", videoCount)
	fmt.Printf("   - cw_task_steps: %d 条\n", stepCount)
}

func benchmarkQuery(db *sql.DB, indexName, query string) {
	iterations := 100

	start := time.Now()
	for i := 0; i < iterations; i++ {
		rows, err := db.Query(query)
		if err != nil {
			fmt.Printf("❌ %s 查询失败: %v\n", indexName, err)
			return
		}
		rows.Close()
	}
	elapsed := time.Since(start)

	avgMs := float64(elapsed.Microseconds()) / float64(iterations) / 1000.0
	fmt.Printf("✅ %-25s: 平均 %.3f ms (共 %d 次)\n", indexName, avgMs, iterations)
}

func explainQuery(db *sql.DB, name, query string) {
	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("❌ %s EXPLAIN 失败: %v\n", name, err)
		return
	}
	defer rows.Close()

	fmt.Printf("\n🔍 %s:\n", name)
	for rows.Next() {
		var id, parent, notused int
		var detail string
		rows.Scan(&id, &parent, &notused, &detail)
		fmt.Printf("   %s\n", detail)
	}
}

func cleanupTestData(db *sql.DB) {
	db.Exec("DELETE FROM cw_saved_videos WHERE video_id LIKE 'test_video_%'")
	db.Exec("DELETE FROM cw_task_steps WHERE video_id LIKE 'test_video_%'")
}
