// Command speako-cli is the teacher's back door for the rare operations that do
// not justify a screen: migrations, seeding, manual credit grants, refunds and
// lead export.
//
// ponytail: a flat switch over os.Args is enough for six verbs; reach for a CLI
// library only if subcommand flags start nesting.
package main

import (
	"fmt"
	"os"
)

const usage = `speako-cli <command>

  migrate                          apply pending database migrations
  seed                             reset dev data: teacher, availability, products
  grant-credit <email> <n> [why]   add credits to a student's ledger
  refund <order-id>                mark an order refunded and revoke what it granted
  export-leads [path.csv]          dump confirmed leads to CSV
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "migrate", "seed", "grant-credit", "refund", "export-leads":
		// S4 implements these against the sqlite adapter.
		err = fmt.Errorf("%s: not implemented yet (session S4)", os.Args[1])
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
