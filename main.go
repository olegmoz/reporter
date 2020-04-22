package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v31/github"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"
)

// GitHub client
var client *github.Client

// GitHub network context
var ctx context.Context

func main() {
	app := cli.App{
		Name:                 "reporter",
		Usage:                "GitHub report generator and statistics aggregator",
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "token",
				Usage: "GitHub API token with",
			},
		},
		Before: setup,
		Commands: []*cli.Command{
			&cli.Command{
				Name:    "report",
				Aliases: []string{"rep"},
				Usage:   "Generate report for period",
				Action:  cmdRep,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "period",
						Aliases: []string{"p"},
						Value:   "daily",
						Usage:   "Report period: either daily or weekly",
					},
					&cli.StringFlag{
						Name:  "date",
						Usage: "date of report",
					},
					&cli.StringFlag{
						Name:  "author",
						Usage: "Filter by author",
					},
				},
			},
			&cli.Command{
				Name:    "status",
				Aliases: []string{"stat", "stats"},
				Usage:   "Show status of project",
				Action:  cmdStat,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "author",
						Usage: "Filter PR by author (submitter)",
					},
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func setup(app *cli.Context) error {
	ctx = context.Background()
	tkn, err := token(app)
	if err != nil {
		return err
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: tkn},
	)
	tc := oauth2.NewClient(ctx, ts)
	client = github.NewClient(tc)
	return nil
}

func token(app *cli.Context) (string, error) {
	token := app.String("token")
	if token != "" {
		return token, nil
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t, nil
	}
	file := os.Getenv("HOME") + "/.config/reporter/github_token.txt"
	if token, err := ioutil.ReadFile(file); err == nil {
		return string(token), nil
	}
	return "", fmt.Errorf("GitHub token neither given as a flag, nor found in env, not in %s", file)
}

func repos(src string) ([]*github.Repository, error) {
	parts := strings.Split(src, "/")
	if len(parts) == 1 {
		repos, _, err := client.Repositories.ListByOrg(ctx, parts[0], nil)
		return repos, err
	} else if len(parts) == 2 {
		rep, _, err := client.Repositories.Get(ctx, parts[0], parts[1])
		if err != nil {
			return nil, err
		}
		return []*github.Repository{rep}, nil
	} else {
		return nil, fmt.Errorf("Unexpected source string: %s", src)
	}
}

func author(app *cli.Context) (string, error) {
	author := app.String("author")
	if author == "me" {
		user, _, err := client.Users.Get(ctx, "")
		if err != nil {
			return "", err
		}
		author = user.GetLogin()
	}
	if author[0] == '@' {
		author = author[1:]
	}
	if author != "" {
		author = strings.ToLower(author)
	}
	return author, nil
}

func cmdStat(app *cli.Context) error {
	repos, err := repos(app.Args().First())
	if err != nil {
		return err
	}
	author, err := author(app)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		prs, _, err := client.PullRequests.List(ctx, repo.GetOwner().GetLogin(), repo.GetName(),
			&github.PullRequestListOptions{State: "open", Sort: "updated"})
		if err != nil {
			return err
		}
		for _, pr := range prs {
			if author != "" && author != strings.ToLower(pr.GetUser().GetLogin()) {
				continue
			}
			if pr.GetDraft() {
				continue
			}
			state := pr.GetState()
			if pr.GetMerged() {
				state += ":merged"
			}
			var assignee string
			if a := pr.GetAssignee(); a != nil {
				assignee = fmt.Sprintf("(a:%s)", a.GetLogin())
			} else {
				assignee = "(a:0)"
			}
			fmt.Printf("PR[%s]: (%s) %s#%d by @%s %s %s\n",
				pr.GetTitle(),
				state,
				repo.GetFullName(), pr.GetNumber(),
				pr.GetUser().GetLogin(), assignee,
				pr.GetHTMLURL())
		}
	}
	return nil
}

func cmdRep(app *cli.Context) error {
	now := time.Now()
	repos, err := repos(app.Args().First())
	if err != nil {
		return err
	}
	author, err := author(app)
	if err != nil {
		return err
	}
	fmt.Println("This is what was merged today:")
	for _, repo := range repos {
		prs, _, err := client.PullRequests.List(ctx, repo.GetOwner().GetLogin(), repo.GetName(),
			&github.PullRequestListOptions{State: "closed"})
		if err != nil {
			return err
		}
		for _, pr := range prs {
			closed := pr.GetClosedAt()
			if compareDate(&now, &closed) != 0 {
				continue
			}
			if author != "" && author != strings.ToLower(pr.GetUser().GetLogin()) {
				continue
			}
			fmt.Printf(" - %s by @%s: %s\n",
				pr.GetTitle(),
				pr.GetUser().GetLogin(),
				pr.GetHTMLURL())
		}
	}
	return nil
}

func compareDate(left, right *time.Time) int {
	ly, lm, ld := left.Date()
	ry, rm, rd := right.Date()
	if ly < ry {
		return 1
	}
	if ly > ry {
		return -1
	}
	if lm < rm {
		return 1
	}
	if lm > rm {
		return -1
	}
	if ld < rd {
		return 1
	}
	if ld > rd {
		return -1
	}
	return 0
}
