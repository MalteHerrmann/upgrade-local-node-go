package gov

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/MalteHerrmann/upgrade-local-node-go/utils"
	abcitypes "github.com/cometbft/cometbft/abci/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/pkg/errors"
)

// buildUpgradeProposalCommand builds the command to submit a software upgrade proposal.
func buildUpgradeProposalCommand(targetVersion string, upgradeHeight int) []string {
	return []string{
		"tx", "gov", "submit-legacy-proposal", "software-upgrade", targetVersion,
		"--title", fmt.Sprintf("'Upgrade to %s'", targetVersion),
		"--description", fmt.Sprintf("'Upgrade to %s'", targetVersion),
		"--upgrade-height", fmt.Sprintf("%d", upgradeHeight),
		"--deposit", "100000000000000000000aevmos",
		"--output", "json",
		"--no-validate",
	}
}

// GetProposalIDFromSubmitEvents looks for the proposal submission event in the given transaction events
// and returns the proposal id, if found.
func GetProposalIDFromSubmitEvents(events []abcitypes.Event) (int, error) {
	for _, event := range events {
		if event.Type != "submit_proposal" {
			continue
		}

		for _, attribute := range event.Attributes {
			if attribute.Key == "proposal_id" {
				proposalID, err := strconv.Atoi(attribute.Value)
				if err != nil {
					return 0, fmt.Errorf("error parsing proposal id: %w", err)
				}

				return proposalID, nil
			}
		}
	}

	return 0, fmt.Errorf("proposal submission event not found")
}

// QueryLatestProposalID queries the latest proposal ID.
func QueryLatestProposalID(bin *utils.Binary) (int, error) {
	out, err := utils.ExecuteBinaryCmd(bin, utils.BinaryCmdArgs{
		Subcommand: []string{"q", "gov", "proposals", "--output=json"},
	})
	if err != nil {
		return 0, errors.Wrap(err, "error querying proposals")
	}

	var res govtypes.QueryProposalsResponse

	err = bin.Cdc.UnmarshalJSON([]byte(out), &res)
	if err != nil {
		return 0, errors.Wrap(err, "error unmarshalling proposals")
	}

	if len(res.Proposals) == 0 {
		return 0, errors.New("no proposals found")
	}

	return int(res.Proposals[len(res.Proposals)-1].ProposalId), nil
}

// SubmitAllVotesForProposal submits a vote for the given proposal ID using all testing accounts.
func SubmitAllVotesForProposal(bin *utils.Binary, proposalID int) {
	accsWithDelegations, err := utils.FilterAccountsWithDelegations(bin)
	if err != nil {
		log.Fatalf("Error filtering accounts: %v", err)
	}

	utils.Wait(1)
	log.Println("Voting for upgrade...")

	for _, acc := range accsWithDelegations {
		if err = VoteForProposal(bin, proposalID, acc.Name); err != nil {
			log.Printf("  - could NOT vote using key: %s\n", acc.Name)
		} else {
			log.Printf("  - voted using key: %s\n", acc.Name)
		}
	}
}

// SubmitUpgradeProposal submits a software upgrade proposal with the given target version and upgrade height.
func SubmitUpgradeProposal(bin *utils.Binary, targetVersion string, upgradeHeight int) (int, error) {
	upgradeProposal := buildUpgradeProposalCommand(targetVersion, upgradeHeight)

	out, err := utils.ExecuteBinaryCmd(bin, utils.BinaryCmdArgs{
		Subcommand:  upgradeProposal,
		From:        "dev0",
		UseDefaults: true,
	})
	if err != nil {
		return 0, errors.Wrap(err,
			fmt.Sprintf("failed to submit upgrade proposal to %s at height %d", targetVersion, upgradeHeight),
		)
	}

	// Clean gas estimate output and only leave json output
	out = strings.TrimSpace(out)
	lines := strings.Split(out, "\n")
	out = lines[len(lines)-1] // last line is json output

	events, err := utils.GetTxEvents(bin, out)
	if err != nil {
		return 0, fmt.Errorf("error getting tx events: %w", err)
	}

	return GetProposalIDFromSubmitEvents(events)
}

// VoteForProposal votes for the proposal with the given ID using the given account.
func VoteForProposal(bin *utils.Binary, proposalID int, sender string) error {
	_, err := utils.ExecuteBinaryCmd(bin, utils.BinaryCmdArgs{
		Subcommand:  []string{"tx", "gov", "vote", fmt.Sprintf("%d", proposalID), "yes"},
		From:        sender,
		UseDefaults: true,
		Quiet:       true,
	})

	return errors.Wrap(err, fmt.Sprintf("failed to vote for proposal %d", proposalID))
}
