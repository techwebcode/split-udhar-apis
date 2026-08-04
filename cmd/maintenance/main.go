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
