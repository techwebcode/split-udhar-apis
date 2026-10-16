// Command maintenance runs data-integrity checks against the SplitUdhar
// database. It is read-only unless -apply is passed.
//
//	go run ./cmd/maintenance -task=recompute-balances          # report only
//	go run ./cmd/maintenance -task=recompute-balances -apply   # correct drift
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"split-udhar-apis/config"
	"split-udhar-apis/migrations"
	"split-udhar-apis/services"

	"gorm.io/gorm"
)

func main() {
	task := flag.String("task", "list", "task to run; -task=list shows the available tasks")
	apply := flag.Bool("apply", false, "write changes; omit for a dry run")
	flag.Parse()

	if *task == "list" {
		fmt.Print(usage)
		return
	}

	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	if !*apply {
		log.Println("DRY RUN — no changes will be written. Re-run with -apply to commit.")
	}

	switch *task {
	case "recompute-balances":
		err = recomputeBalances(db, *apply)
	case "migrate-user-devices":
		err = migrateUserDevices(db, *apply)
	case "rollback-user-devices":
		err = migrations.RollbackUserDevicesSchema(db)
	default:
		fmt.Fprintf(os.Stderr, "unknown task %q\n\n%s", *task, usage)
		os.Exit(2)
	}

	if err != nil {
		log.Fatalf("task %s failed: %v", *task, err)
	}

	if !*apply {
		log.Println("DRY RUN complete — nothing was written.")
	}
}

const usage = `Available tasks:

  recompute-balances   Replay every group's expenses and settlements and
                       report any group_members.balance that has drifted from
                       the ledger. Corrects the drift when -apply is passed.

  migrate-user-devices Preflight-check and safely optimize user_devices schema
                       (fcm_token VARCHAR(255) and composite indexes).
                       Applies migration when -apply is passed.

  rollback-user-devices Revert user_devices composite indexes and restore
                       fcm_token to VARCHAR(500).

Flags:
  -apply   Write changes. Without it the task only reports.
`

func recomputeBalances(db *gorm.DB, apply bool) error {
	report, err := services.NewGroupService(db).RecomputeGroupBalances(apply)
	if err != nil {
		return err
	}

	log.Printf("recompute-balances: scanned %d group(s)", report.GroupsScanned)

	for _, skipped := range report.Skipped {
		log.Printf("  ! skipped %s", skipped)
	}

	if len(report.Drifts) == 0 {
		log.Println("recompute-balances: all balances already match the ledger")
		return nil
	}

	log.Printf("recompute-balances: %d member balance(s) drifted", len(report.Drifts))
	for _, d := range report.Drifts {
		log.Printf(
			"  group %d (%s) member %s: stored %.2f, expected %.2f (%+.2f)",
			d.GroupID, d.GroupName, d.UserMobile, d.Stored, d.Expected, d.Delta(),
		)
	}

	if apply {
		log.Printf("recompute-balances: corrected %d balance(s)", len(report.Drifts))
	}
	return nil
}

func migrateUserDevices(db *gorm.DB, apply bool) error {
	report, err := migrations.MigrateUserDevicesSchema(db, apply)
	if err != nil {
		return err
	}
	log.Printf("migrate-user-devices: %s", report.ActionTaken)
	log.Printf("  total devices: %d (active: %d)", report.TotalDevices, report.ActiveDevices)
	log.Printf("  max token length: %d chars", report.MaxTokenLength)
	if len(report.IndexesCreated) > 0 {
		log.Printf("  created indexes: %v", report.IndexesCreated)
	}
	if len(report.IndexesExisting) > 0 {
		log.Printf("  already existing indexes: %v", report.IndexesExisting)
	}
	return nil
}
