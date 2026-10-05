package templates

// GetDefaultTemplates returns the full catalog of curated templates.
func GetDefaultTemplates() []ScheduledTemplate {
	templates := []ScheduledTemplate{
		// -------------------------------------------------------------
		// Category 1: Personal Assistant & Daily Life (For Ordinary People)
		// -------------------------------------------------------------
		{
			ID:       "daily-news-feed-digest",
			Title:    "Daily News & Feed Digest",
			Subtitle: "Automated morning news brief from selected feeds & sources",
			Category: "Personal Assistant",
			Icon:     "Newspaper",
			Description: "Every morning, fetches the latest headlines and articles from specified news sites, RSS feeds, and tech newsletters, producing a concise executive summary with key takeaways and source links.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "daily",
				TimeOfDay:      "08:00",
				DaysOfWeek:     []int{1, 2, 3, 4, 5, 6, 7},
				CronExpression: "0 8 * * *",
			},
			RequiredTools:  []string{"fetch", "tavily", "markitdown"},
			RequiredSkills: []string{"research"},
			Parameters: []TemplateParameter{
				{
					Key:          "sources",
					Label:        "News Sources & Topics",
					Type:         "textarea",
					DefaultValue: "Hacker News top stories, TechCrunch AI developments, Bloomberg Technology market headlines, Reuters world business",
					Description:  "Enter URLs, RSS feeds, or news topics to monitor every morning.",
				},
				{
					Key:          "max_items",
					Label:        "Max Items per Section",
					Type:         "text",
					DefaultValue: "5",
					Description:  "Number of top curated stories to include in the briefing.",
				},
			},
			PromptTemplate: `Task: Produce today's Morning News & Feed Digest.

Sources and topics to review:
{{sources}}

Instructions:
1. Fetch and search the latest articles and breaking updates published within the last 24 hours.
2. Group stories into:
   - Top Global & Tech Headlines
   - AI & Engineering Developments
   - Market & Business Trends
3. For each story, provide:
   - A bold 1-sentence headline
   - A 2-sentence summary explaining why it matters
   - Source link
4. Format the final output in clean, readable GitHub-flavored markdown. Keep it crisp and executive-ready.`,
		},

		{
			ID:       "portfolio-market-sentinel",
			Title:    "Portfolio & Market Sentinel",
			Subtitle: "Daily market-close portfolio tracking & macro alert sentinel",
			Category: "Personal Assistant",
			Icon:     "TrendingUp",
			Description: "Runs at market close to summarize the performance of your crypto, equity, and ETF holdings. Flags abnormal price swings (>3%) and highlights macro catalysts.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "daily",
				TimeOfDay:      "17:00",
				DaysOfWeek:     []int{1, 2, 3, 4, 5},
				CronExpression: "0 17 * * 1-5",
			},
			RequiredTools:  []string{"ccxt", "financial-datasets", "alphavantage"},
			RequiredSkills: []string{"research"},
			Parameters: []TemplateParameter{
				{
					Key:          "tickers",
					Label:        "Watchlist Symbols",
					Type:         "text",
					DefaultValue: "BTC/USDT, ETH/USDT, SOL/USDT, NVDA, AAPL, MSFT, SPY, QQQ",
					Description:  "Comma-separated ticker symbols to track.",
				},
				{
					Key:          "volatility_threshold",
					Label:        "Alert Volatility Threshold (%)",
					Type:         "text",
					DefaultValue: "3.0",
					Description:  "Percentage move (up or down) to highlight as significant alert.",
				},
			},
			PromptTemplate: `Task: Daily Portfolio and Market Performance Review.

Watchlist:
{{tickers}}
Alert Threshold: {{volatility_threshold}}%

Instructions:
1. Query current closing prices and 24h/daily price changes for all listed tickers.
2. Calculate overall market sentiment and index benchmarks (SPY, QQQ, BTC).
3. Identify any asset moving more than {{volatility_threshold}}% and investigate the key catalyst or earnings release driving the movement.
4. Output a clean summary table:
   | Ticker | Price | 24h Change | Status / Alert |
5. Add a 3-bullet executive summary highlighting key takeaways and upcoming earnings or economic releases for tomorrow.`,
		},

		{
			ID:       "calendar-day-organizer",
			Title:    "Daily Calendar & Agenda Organizer",
			Subtitle: "Morning time-blocking, meeting prep & priority scheduling",
			Category: "Personal Assistant",
			Icon:     "CalendarCheck",
			Description: "Runs every morning at 7:30 AM to inspect your schedule for the day. Organizes your timetable, reserves focus blocks, and prepares briefing notes for upcoming meetings.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "daily",
				TimeOfDay:      "07:30",
				DaysOfWeek:     []int{1, 2, 3, 4, 5},
				CronExpression: "30 7 * * 1-5",
			},
			RequiredTools:  []string{"structured", "time"},
			RequiredSkills: []string{"research"},
			Parameters: []TemplateParameter{
				{
					Key:          "focus_hours",
					Label:        "Target Deep Work Hours",
					Type:         "text",
					DefaultValue: "3",
					Description:  "Target hours of uninterrupted deep work to block out.",
				},
				{
					Key:          "prep_buffer",
					Label:        "Meeting Prep Buffer (minutes)",
					Type:         "text",
					DefaultValue: "15",
					Description:  "Minutes of preparation buffer before critical meetings.",
				},
			},
			PromptTemplate: `Task: Organize and Plan Today's Agenda.

Target Deep Work: {{focus_hours}} hours
Meeting Prep Buffer: {{prep_buffer}} minutes

Instructions:
1. Review today's events, deadlines, and pending tasks in Structured / calendar.
2. Identify the Top 3 Most Important Tasks (MITs) that must be completed today.
3. Construct an optimized hourly schedule from 08:30 to 18:00 with realistic buffers and deep-work blocks.
4. For every external meeting, note 2-3 preparation bullet points or questions to review.
5. Provide a clear, motivating morning overview.`,
		},

		{
			ID:       "evening-standup-journal",
			Title:    "Evening Standup & Work Log",
			Subtitle: "Automated work digest and daily milestone reflection",
			Category: "Personal Assistant",
			Icon:     "CheckSquare",
			Description: "Runs every evening at 6:00 PM to scan git commits, modified workspace files, and completed tasks, writing a clean daily standup note into your project documentation.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "daily",
				TimeOfDay:      "18:00",
				DaysOfWeek:     []int{1, 2, 3, 4, 5},
				CronExpression: "0 18 * * 1-5",
			},
			RequiredTools:  []string{"github", "structured"},
			RequiredSkills: []string{"code-review"},
			Parameters: []TemplateParameter{
				{
					Key:          "repos",
					Label:        "Repositories / Projects",
					Type:         "text",
					DefaultValue: "All active workspaces in Antigravity",
					Description:  "Projects to include in the daily accomplishment digest.",
				},
			},
			PromptTemplate: `Task: Compile Evening Standup & Daily Work Log.

Projects: {{repos}}

Instructions:
1. Inspect git log for commits authored today across the repositories.
2. Group accomplishments into:
   - What was accomplished today (features delivered, bugs fixed, docs written)
   - What is currently in-progress
   - Blockers or open items needing attention tomorrow
3. Generate a concise, professional markdown standup note formatted for easy pasting into team channels or personal journals.`,
		},

		// -------------------------------------------------------------
		// Category 2: CI/CD & Development (Devin / Codex / Claude Code)
		// -------------------------------------------------------------
		{
			ID:       "fix-ci-failures",
			Title:    "Fix CI & Build Failures",
			Subtitle: "Automated root-cause analysis and patch creation for broken builds",
			Category: "CI/CD & Development",
			Icon:     "GitPullRequest",
			Description: "Continuously or periodically scans repository pull requests and CI pipelines. When a check fails, pulls the failure logs, reproduces the issue locally, applies a targeted patch, and verifies test passes.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "hourly",
				TimeOfDay:      "00:00",
				DaysOfWeek:     []int{1, 2, 3, 4, 5, 6, 7},
				CronExpression: "0 * * * *",
			},
			RequiredTools:  []string{"github"},
			RequiredSkills: []string{"diagnosing-bugs", "tdd"},
			Parameters: []TemplateParameter{
				{
					Key:          "repo",
					Label:        "Target Repository",
					Type:         "text",
					DefaultValue: "Current workspace repository",
					Description:  "Repository to monitor for failing workflows.",
				},
				{
					Key:          "branch",
					Label:        "Target Branch",
					Type:         "text",
					DefaultValue: "main",
					Description:  "Branch to monitor or create fix branches from.",
				},
			},
			PromptTemplate: `Task: Inspect and Resolve Failing CI Checks.

Repository: {{repo}}
Branch: {{branch}}

Instructions:
1. Check the latest workflow runs and test runs. If all checks are passing, conclude with a clean status message.
2. If any check or unit test failed:
   - Download and parse the error log to extract the exact stack trace and failing test name.
   - Locate the source file and diagnose the root cause.
   - Formulate a clean, minimal code fix following the project's coding standards.
   - Run tests locally to ensure 100% green pass.
   - Commit the fix or create a dedicated pull request explaining the cause and solution.`,
		},

		{
			ID:       "weekly-dependency-update",
			Title:    "Weekly Dependency & Security Update",
			Subtitle: "Automated package upgrades with breaking-change auditing",
			Category: "CI/CD & Development",
			Icon:     "PackageCheck",
			Description: "Every Monday morning, scans dependencies across package.json, go.mod, and requirements.txt for available updates. Reviews changelogs for breaking changes, runs tests, and creates an upgrade PR.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "weekly",
				TimeOfDay:      "09:00",
				DaysOfWeek:     []int{1},
				CronExpression: "0 9 * * 1",
			},
			RequiredTools:  []string{"github"},
			RequiredSkills: []string{"code-review"},
			Parameters: []TemplateParameter{
				{
					Key:          "upgrade_scope",
					Label:        "Upgrade Scope",
					Type:         "select",
					DefaultValue: "minor_and_patch",
					Description:  "Allowed upgrade level: patch only, minor and patch, or all.",
				},
			},
			PromptTemplate: `Task: Weekly Dependency & Security Update Routine.

Scope: {{upgrade_scope}}

Instructions:
1. Inspect dependency manifests in the workspace (package.json, go.mod, requirements.txt).
2. Check for outdated packages and known security advisories (CVEs).
3. Increment packages within safe minor/patch boundaries.
4. Execute full build and unit test suite to verify no regressions or breaks.
5. Generate a categorized summary of updated libraries with links to their changelogs.`,
		},

		{
			ID:       "weekly-changelog-generator",
			Title:    "Weekly Changelog & Release Notes",
			Subtitle: "Compiles merged pull requests into categorized release digests",
			Category: "CI/CD & Development",
			Icon:     "FileText",
			Description: "Every Friday afternoon, compiles all pull requests merged in the past 7 days into a beautifully formatted changelog categorized into Features, Bug Fixes, Performance, and Documentation.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "weekly",
				TimeOfDay:      "17:00",
				DaysOfWeek:     []int{5},
				CronExpression: "0 17 * * 5",
			},
			RequiredTools:  []string{"github"},
			RequiredSkills: []string{"writing-for-agents"},
			Parameters: []TemplateParameter{
				{
					Key:          "release_heading",
					Label:        "Release Heading / Tag",
					Type:         "text",
					DefaultValue: "Weekly Development Digest",
					Description:  "Title for this week's release changelog.",
				},
			},
			PromptTemplate: `Task: Generate Weekly Changelog and Release Notes.

Heading: {{release_heading}}

Instructions:
1. Query all merged pull requests and commit messages from the last 7 days.
2. Group items into:
   - 🚀 New Features & Enhancements
   - 🐛 Bug Fixes & Stability
   - ⚡ Performance & Optimizations
   - 🛠 Refactoring & Internal Maintenance
3. Provide clickable links to PRs and author acknowledgments where applicable.
4. Output in clean GitHub-Flavored Markdown ready for release or team announcement.`,
		},

		{
			ID:       "stale-pr-cleanup",
			Title:    "Stale PR & Issue Triage Cleaner",
			Subtitle: "Monitors inactive branches, merge conflicts, and unattended issues",
			Category: "CI/CD & Development",
			Icon:     "GitMerge",
			Description: "Scans open pull requests and issues inactive for more than 14 days. Checks for merge conflicts against the default branch and drafts courteous status pings for assignees.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "weekly",
				TimeOfDay:      "10:00",
				DaysOfWeek:     []int{3},
				CronExpression: "0 10 * * 3",
			},
			RequiredTools:  []string{"github"},
			RequiredSkills: []string{"code-review"},
			Parameters: []TemplateParameter{
				{
					Key:          "inactivity_days",
					Label:        "Inactivity Threshold (Days)",
					Type:         "text",
					DefaultValue: "14",
					Description:  "Number of days without activity before flagging a PR or issue.",
				},
			},
			PromptTemplate: `Task: Stale Pull Request & Issue Triage.

Inactivity Threshold: {{inactivity_days}} days

Instructions:
1. List open PRs and issues with no activity for more than {{inactivity_days}} days.
2. Check if each stale PR has merge conflicts with the primary branch.
3. Formulate a triage summary table:
   | Type | Number | Title | Inactive Days | Mergeable | Recommended Action |
4. Flag items that should be closed, rebased, or escalated for review.`,
		},

		// -------------------------------------------------------------
		// Category 3: Security & Code Health
		// -------------------------------------------------------------
		{
			ID:       "secret-scanner-routine",
			Title:    "Secret & Leaked Credential Scanner",
			Subtitle: "Daily sentinel against committed API keys, tokens & credentials",
			Category: "Security & Quality",
			Icon:     "ShieldAlert",
			Description: "Daily automated scan across recent commits and modified workspace files for inadvertently committed API keys (sk-..., gh_token, JWTs, AWS credentials, and unignored .env files).",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "daily",
				TimeOfDay:      "02:00",
				DaysOfWeek:     []int{1, 2, 3, 4, 5, 6, 7},
				CronExpression: "0 2 * * *",
			},
			RequiredTools:  []string{"github"},
			RequiredSkills: []string{"code-review"},
			Parameters: []TemplateParameter{
				{
					Key:          "scan_depth",
					Label:        "Scan Commit Depth",
					Type:         "text",
					DefaultValue: "50",
					Description:  "Number of recent commits to audit.",
				},
			},
			PromptTemplate: `Task: Security & Secret Leak Prevention Audit.

Commit Depth: {{scan_depth}}

Instructions:
1. Scan recent commits and unstaged files for known credential patterns:
   - OpenAI, Anthropic, Google API keys
   - AWS, GCP, Azure secret credentials
   - Private keys (id_rsa, PEM certificates)
   - Database connection strings with embedded passwords
2. If any potential leak is detected, isolate the file and commit hash immediately.
3. Recommend safe remediation steps (key revocation, git history rewriting via filter-repo).
4. Provide a clear clean bill of health or vulnerability alert report.`,
		},

		{
			ID:       "code-pattern-enforcer",
			Title:    "Code Pattern & Architecture Enforcer",
			Subtitle: "Automated architectural consistency and deep module auditing",
			Category: "Security & Quality",
			Icon:     "CheckCircle",
			Description: "Reviews code changes against architectural standards (deep modules, clean seams, interface segregation, error handling conventions), flagging antipatterns before they compound.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "weekly",
				TimeOfDay:      "11:00",
				DaysOfWeek:     []int{2},
				CronExpression: "0 11 * * 2",
			},
			RequiredTools:  []string{"github"},
			RequiredSkills: []string{"codebase-design", "code-review"},
			Parameters: []TemplateParameter{
				{
					Key:          "standards_file",
					Label:        "Standards Reference File",
					Type:         "text",
					DefaultValue: "PROJECT.md",
					Description:  "File documenting architectural rules and conventions.",
				},
			},
			PromptTemplate: `Task: Code Pattern & Architecture Review.

Standards Source: {{standards_file}}

Instructions:
1. Examine code changes introduced over the past week against the architecture standards documented in {{standards_file}}.
2. Check for:
   - Deep module boundaries (simple interfaces, deep implementations)
   - Proper error propagation and domain error wrapping
   - Absence of duplicate abstractions or leaking implementation details
3. Produce a constructive report with code snippets highlighting areas of exemplary design and specific recommendations for refactoring.`,
		},

		// -------------------------------------------------------------
		// Category 4: Research & Market Intelligence
		// -------------------------------------------------------------
		{
			ID:       "arxiv-research-radar",
			Title:    "arXiv & Academic Literature Radar",
			Subtitle: "Weekly digest of cutting-edge papers in your focus domain",
			Category: "Research & Market",
			Icon:     "BookOpen",
			Description: "Every Monday, queries arXiv and scientific databases for breakthrough research matching your focus topics. Produces 1-page executive summaries of the top 3 papers.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "weekly",
				TimeOfDay:      "08:30",
				DaysOfWeek:     []int{1},
				CronExpression: "30 8 * * 1",
			},
			RequiredTools:  []string{"literature-search-arxiv", "pubmed-database"},
			RequiredSkills: []string{"literature-search-arxiv"},
			Parameters: []TemplateParameter{
				{
					Key:          "query_keywords",
					Label:        "Research Topics / Keywords",
					Type:         "textarea",
					DefaultValue: "LLM agent reasoning, autonomous multi-agent coordination, reinforcement learning from verifiable rewards, financial market making algorithms",
					Description:  "Keywords and categories to scan on arXiv.",
				},
			},
			PromptTemplate: `Task: Curate Weekly Academic Research Radar.

Topics: {{query_keywords}}

Instructions:
1. Search arXiv for papers submitted in the last 7 days matching the keywords.
2. Select the Top 3 most impactful or novel preprints.
3. For each selected paper, provide:
   - Title, Authors, and arXiv link
   - Core Problem: What challenge does the paper solve?
   - Key Innovation: What novel architecture or method is proposed?
   - Empirical Results: Key benchmark figures or speedups
   - Practical Implications: How this can apply to our own projects
4. Format in clean, readable academic briefing markdown.`,
		},

		{
			ID:       "competitor-landscape-tracker",
			Title:    "Competitor & Feature Radar",
			Subtitle: "Monitors competitor blogs, changelogs & pricing shifts",
			Category: "Research & Market",
			Icon:     "Eye",
			Description: "Weekly automated crawl across competitor product sites, changelogs, and release announcements to detect newly launched features, pricing changes, and market shifts.",
			DefaultSchedule: TemplateSchedule{
				Frequency:      "weekly",
				TimeOfDay:      "09:00",
				DaysOfWeek:     []int{4},
				CronExpression: "0 9 * * 4",
			},
			RequiredTools:  []string{"tavily", "read_url_content", "fetch"},
			RequiredSkills: []string{"research"},
			Parameters: []TemplateParameter{
				{
					Key:          "competitor_urls",
					Label:        "Competitor URLs / Changelogs",
					Type:         "textarea",
					DefaultValue: "https://cursor.com/changelog, https://devin.ai/blog, https://claude.ai/blog",
					Description:  "Competitor sites and changelog URLs to monitor.",
				},
			},
			PromptTemplate: `Task: Competitor Landscape & Feature Release Audit.

Target URLs:
{{competitor_urls}}

Instructions:
1. Fetch and review the latest content and releases from the listed competitor URLs.
2. Identify:
   - New features launched in the last 1-2 weeks
   - Pricing or tier structure updates
   - Strategic positioning changes
3. Provide a competitive comparison matrix showing our current capabilities vs competitor announcements.
4. Conclude with 2 actionable strategic recommendations for our roadmap.`,
		},
	}

	for i := range templates {
		if templates[i].DefaultSchedule.ScheduleText == "" {
			templates[i].DefaultSchedule.ScheduleText = FormatSchedule(templates[i].DefaultSchedule)
		}
	}
	return templates
}
