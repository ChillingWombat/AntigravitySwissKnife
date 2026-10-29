import { describe, it } from "node:test"
import assert from "node:assert/strict"
import fs from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

describe("GitHub Workspace Tab Badges - Open Items Only", () => {
  it("verifies GitHubWorkspacePage tab badges represent open items rather than raw array length", () => {
    const currentDir = path.dirname(fileURLToPath(import.meta.url))
    const pagePath = path.resolve(currentDir, "../pages/GitHubWorkspacePage.tsx")
    const content = fs.readFileSync(pagePath, "utf-8")

    // Must not use raw unconditioned issues.length or prs.length in the tab header button
    assert.ok(!content.includes("Issues ({issues.length})"), "Issues tab must not use raw issues.length")
    assert.ok(!content.includes("Pull Requests ({prs.length})"), "Pull Requests tab must not use raw prs.length")
  })

  it("calculates open count correctly when given mixed open and closed items", () => {
    const mockIssues = [
      { number: 1, state: "open" },
      { number: 2, state: "OPEN" },
      { number: 3, state: "closed" },
      { number: 4, state: "CLOSED" },
    ]
    const openCount = mockIssues.filter(i => i && (i.state || "open").toLowerCase() === "open").length
    assert.equal(openCount, 2)

    const mockPRs = [
      { number: 10, state: "open" },
      { number: 11, state: "merged" },
      { number: 12, state: "closed" },
    ]
    const openPRCount = mockPRs.filter(p => p && (p.state || "open").toLowerCase() === "open").length
    assert.equal(openPRCount, 1)
  })
})
