import { Agent } from "./types"

/**
 * Issue Resolver Agent Configuration
 *
 * This agent orchestrates the autonomous issue resolution pipeline.
 * It continuously picks open GitHub issues and runs each through
 * Plan → Implement → Validate → Review → Commit → Close.
 *
 * The agent has God-level permissions (same as the host) and operates
 * autonomously until interrupted or no issues remain.
 */

export const IssueResolverAgent: Agent = {
  name: "resolver",
  description: "Autonomous issue resolution pipeline — continuously picks open issues, fixes them, validates, and closes them",
  color: "cyan",
  permission: {
    bash: {
      "*": "allow",
    },
    read: {
      "*": "allow",
    },
    edit: {
      "*": "allow",
    },
    glob: {
      "*": "allow",
    },
    grep: {
      "*": "allow",
    },
    web_search: {
      "*": "allow",
    },
    execute: true,
    memory: true,
    /*** Allow spawning unlimited sub-agents */
    task: {
      "*": "allow",
    },
  },
}
