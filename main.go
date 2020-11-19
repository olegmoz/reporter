package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/caarlos0/spin"
	"github.com/google/go-github/v31/github"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"os"
	"strings"
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
			&cli.StringFlag{
				Name:  "verbose",
				Usage: "Verbose output",
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
					&cli.BoolFlag{
						Name:  "authors",
						Usage: "Show PR authors",
					},
				},
			},
			&cli.Command{
				Name:    "contrib",
				Aliases: []string{"contr"},
				Usage:   "Generate report for contributors statistics",
				Action:  cmdContribs,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "period",
						Aliases: []string{"p"},
						Value:   "daily",
						Usage:   "Report period: either daily or weekly",
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
	if len(author) > 2 && author[0] == '@' {
		author = author[1:]
	}
	if author != "" {
		author = strings.ToLower(author)
	}
	return author, nil
}

func cmdStat(app *cli.Context) error {
	fmt.Println("Active pull requests:")
	s := spin.New(" - %s")
	s.Set(spin.Spin1)
	s.Start()
	defer s.Stop()
	repos, err := repos(app.Args().First())
	if err != nil {
		return err
	}
	author, err := author(app)
	if err != nil {
		return err
	}
	fmt.Print(spin.ClearLine)
	empty := true
	for _, repo := range repos {
		prs, _, err := client.PullRequests.List(ctx, repo.GetOwner().GetLogin(), repo.GetName(),
			&github.PullRequestListOptions{State: "open", Sort: "updated"})
		if err != nil {
			return err
		}
		for _, pr := range prs {
			if strings.HasPrefix(pr.GetUser().GetLogin(), "dependabot") {
				continue
			}
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
				assignee = fmt.Sprintf("(a:@%s)", a.GetLogin())
			} else {
				assignee = "(a:0)"
			}
			revs, _, err := client.PullRequests.ListReviews(ctx, repo.GetOwner().GetLogin(), repo.GetName(),
				pr.GetNumber(), nil)
			if err != nil {
				return err
			}
			revstat := "["
			for _, rev := range revs {
				if rev.GetState() == "DISMISSED" || rev.GetState() == "COMMENTED" {
					continue
				}
				revstat += fmt.Sprintf("%s:%s,", rev.GetUser().GetLogin(), rev.GetState())
			}
			revstat += "]"
			fmt.Print(spin.ClearLine)
			fmt.Printf(" - %s (%s, %s) by @%s %s %s\n",
				pr.GetTitle(),
				state,
				revstat,
				pr.GetUser().GetLogin(), assignee,
				pr.GetHTMLURL())
			empty = false
		}
	}
	if empty {
		fmt.Print(spin.ClearLine)
		fmt.Println(" - None ;)")
	}
	return nil
}

func cmdRep(app *cli.Context) error {
	rng, err := ParseRange(app.String("period"))
	if err != nil {
		return err
	}
	s := spin.New(" - %s")
	s.Set(spin.Spin1)
	s.Start()
	defer s.Stop()
	repos, err := repos(app.Args().First())
	if err != nil {
		return err
	}
	author, err := author(app)
	if err != nil {
		return err
	}
	authors := app.Bool("authors")
	empty := true
	var line bytes.Buffer
	for _, repo := range repos {
		prs, _, err := client.PullRequests.List(ctx, repo.GetOwner().GetLogin(), repo.GetName(),
			&github.PullRequestListOptions{State: "closed"})
		if err != nil {
			return err
		}
		for _, pr := range prs {
			closed := pr.GetClosedAt()
			// don't use pr.Merged since PR list doesn't include this field
			if pr.MergedAt == nil {
				continue
			}
			if !rng.Include(&closed) {
				continue
			}
			if strings.HasPrefix(pr.GetUser().GetLogin(), "dependabot") {
				continue
			}
			if author != "" && author != strings.ToLower(pr.GetUser().GetLogin()) {
				continue
			}
			line.WriteString(" - ")
			line.WriteString(pr.GetTitle())
			if !authors {
				line.WriteString(" @")
				line.WriteString(pr.GetUser().GetLogin())
			}
			line.WriteString(": ")
			line.WriteString(pr.GetHTMLURL())
			fmt.Print(spin.ClearLine)
			fmt.Println(line.String())
			line.Reset()
			empty = false
		}
	}
	if empty {
		fmt.Print(spin.ClearLine)
		fmt.Println(" - Nothing ;)")
	}
	return nil
}

func cmdContribs(app *cli.Context) error {
	rng, err := ParseRange(app.String("period"))
	if err != nil {
		return err
	}
	fmt.Println("Contributors statistics:")
	verbose := app.Bool("verbose")
	s := spin.New(" - %s")
	if !verbose {
		s.Set(spin.Spin1)
		s.Start()
		defer s.Stop()
	}
	repos, err := repos(app.Args().First())
	if err != nil {
		return err
	}
	author, err := author(app)
	if err != nil {
		return err
	}
	stats := usersStats(make(map[string]*userStats))
	for _, repo := range repos {
		prs, _, err := client.PullRequests.List(ctx, repo.GetOwner().GetLogin(), repo.GetName(),
			&github.PullRequestListOptions{State: "closed"})
		if err != nil {
			return err
		}
		for _, pr := range prs {
			if !rng.Include(pr.ClosedAt) {
				continue
			}
			if strings.HasPrefix(pr.GetUser().GetLogin(), "dependabot") {
				continue
			}
			rvs, _, err := client.PullRequests.ListReviews(ctx, repo.GetOwner().GetLogin(),
				repo.GetName(), pr.GetNumber(), nil)
			if err != nil {
				return err
			}
			// check reviews
			reviewers := make(map[string]bool)
			for _, rev := range rvs {
				if rev.GetAuthorAssociation() != "MEMBER" {
					continue
				}
				state := rev.GetState()
				if state != "CHANGES_REQUESTED" && state != "APPROVED" {
					continue
				}
				reviewers[rev.GetUser().GetLogin()] = true
				if verbose && author == "" || (author != "" && author == rev.GetUser().GetLogin()) {
					fmt.Printf("review by %s: %s\n", rev.GetUser().GetLogin(), rev.GetHTMLURL())
				}
			}
			for reviewer := range reviewers {
				if author == "" || (author != "" && author == reviewer) {
					stats.review(reviewer)
				}
			}
			// check PR merge
			if pr.MergedAt != nil {
				user := pr.GetUser().GetLogin()
				if author == "" || (author != "" && author == user) {
					stats.pull(user)
				}
				if verbose && author == "" || (author != "" && author == user) {
					fmt.Printf("PR by %s: %s\n", user, pr.GetHTMLURL())
				}
			}
		}

	}
	tickets, _, err := client.Issues.ListByOrg(ctx, app.Args().First(),
		&github.IssueListOptions{Filter: "assigned", State: "closed"})
	if err != nil {
		return err
	}
	for _, ticket := range tickets {
		if !rng.Include(ticket.ClosedAt) {
			continue
		}
		if ticket.GetAssignee() == nil ||
			ticket.GetAssignee().GetLogin() == ticket.GetUser().GetLogin() {
			continue
		}
		user := ticket.GetUser().GetLogin()
		if author == "" || (author != "" && author == user) {
			stats.issue(user)
		}
		if verbose && author == "" || (author != "" && author == user) {
			fmt.Printf("Issue by %s: %s\n", user, ticket.GetHTMLURL())
		}

	}
	s.Stop()
	fmt.Print(spin.ClearLine)
	for name, stats := range stats {
		fmt.Printf("%s - %s (%f)\n", name, stats, stats.sum())
	}
	return nil
}

type usersStats map[string]*userStats

func (s usersStats) get(user string) *userStats {
	res := s[user]
	if res == nil {
		res = new(userStats)
		s[user] = res
	}
	return res
}

func (s usersStats) review(name string) {
	us := s.get(name)
	us.Reviews++
}

func (s usersStats) pull(name string) {
	us := s.get(name)
	us.Pulls++
}

func (s usersStats) issue(name string) {
	us := s.get(name)
	us.Issues++
}

type userStats struct {
	Pulls   uint
	Issues  uint
	Reviews uint
}

func (s *userStats) String() string {
	return fmt.Sprintf("pr=%d rev=%d tic=%d", s.Pulls, s.Reviews, s.Issues)
}

func (s *userStats) sum() float32 {
	return float32(s.Pulls) + float32(s.Reviews)*0.5 + float32(s.Issues)*0.5
}
