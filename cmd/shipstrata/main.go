// Command shipstrata 启动船舶沉没遗址层位关系复核台服务，或执行端到端冒烟测试。
//
//   - 默认启动 HTTP 服务（--addr，库前缀 /api）。
//   - --smoke-test：真实建实体 -> 确认接触 -> 重新求解 -> 发布剖面 -> 关闭重开同库
//     验证持久化 -> 成功则退出 0。用于健康基线与门禁自测。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"task242-shipstrata/internal/contact"
	"task242-shipstrata/internal/httpapi"
	"task242-shipstrata/internal/profile"
	"task242-shipstrata/internal/service"
	"task242-shipstrata/internal/site"
	"task242-shipstrata/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP 监听地址")
	dbPath := flag.String("db", "shipstrata.db", "SQLite 数据库路径")
	smoke := flag.Bool("smoke-test", false, "运行端到端冒烟测试，成功后退出 0")
	flag.Parse()

	if *smoke {
		if err := runSmoke(*dbPath); err != nil {
			log.Fatalf("smoke-test failed: %v", err)
		}
		fmt.Println("smoke-test OK")
		os.Exit(0)
	}

	st, err := store.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	svc := service.New(st)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.NewServer(svc).Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("shipstrata listening on %s (db=%s)", *addr, *dbPath)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

// runSmoke 真实写入并重启验证持久化，覆盖核心闭环。
func runSmoke(dbPath string) error {
	ctx := context.Background()
	_ = os.Remove(dbPath)

	st, err := store.OpenStore(dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	svc := service.New(st)

	// 1) 创建遗址与单元
	sb, err := site.Create(st, "TST-001", "Smoke Site", "offshore")
	if err != nil {
		return fmt.Errorf("create site: %w", err)
	}
	u1, err := site.CreateUnit(st, sb.ID, "Hull A", "component", 10, 12, "")
	if err != nil {
		return fmt.Errorf("create unit: %w", err)
	}
	u2, err := site.CreateUnit(st, sb.ID, "Ballast", "sediment", 8, 10, "")
	if err != nil {
		return fmt.Errorf("create unit: %w", err)
	}
	u3, err := site.CreateUnit(st, sb.ID, "Sea floor", "sediment", 0, 8, "")
	if err != nil {
		return fmt.Errorf("create unit: %w", err)
	}

	// 2) 导入并确认接触关系（覆盖/切割均表示上覆晚于下伏）
	_, c1, err := contact.Import(st, sb.ID, u2.ID, u3.ID, "overlies", "survey1", 1, "")
	if err != nil {
		return fmt.Errorf("import contact1: %w", err)
	}
	_, c2, err := contact.Import(st, sb.ID, u1.ID, u2.ID, "cuts", "survey1", 2, "")
	if err != nil {
		return fmt.Errorf("import contact2: %w", err)
	}
	if _, err := contact.Confirm(st, c1.ID); err != nil {
		return fmt.Errorf("confirm contact1: %w", err)
	}
	if _, err := contact.Confirm(st, c2.ID); err != nil {
		return fmt.Errorf("confirm contact2: %w", err)
	}

	// 3) 重新求解并持久化矛盾/侵扰候选
	if _, err := svc.Strata.Reconcile(ctx, sb.ID); err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}

	// 4) 发布剖面版本
	if _, _, err := svc.Profile.Publish(ctx, sb.ID, profile.PublishInput{Label: "v1", Author: "tester"}); err != nil {
		return fmt.Errorf("publish profile: %w", err)
	}

	// 5) 关闭后重开同库，验证持久化
	st.Close()
	st2, err := store.OpenStore(dbPath)
	if err != nil {
		return fmt.Errorf("reopen store: %w", err)
	}
	defer st2.Close()

	us, err := st2.ListUnits(sb.ID)
	if err != nil {
		return fmt.Errorf("list units after reopen: %w", err)
	}
	if len(us) < 3 {
		return fmt.Errorf("persistence check failed: got %d units, want >=3", len(us))
	}
	cs, err := st2.ListContacts(sb.ID)
	if err != nil {
		return fmt.Errorf("list contacts after reopen: %w", err)
	}
	if len(cs) < 2 {
		return fmt.Errorf("persistence check failed: got %d contacts, want >=2", len(cs))
	}
	ps, err := st2.ListProfiles(sb.ID)
	if err != nil {
		return fmt.Errorf("list profiles after reopen: %w", err)
	}
	if len(ps) < 1 {
		return fmt.Errorf("persistence check failed: got %d profiles, want >=1", len(ps))
	}
	return nil
}
