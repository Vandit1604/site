---
title: "How to Contribute to Open Source"  
date: "2024-9-19"  
tags: ["tech"]  
---

> **Disclaimer:** Contributing to open source requires patience—lots of it. If you lack the time or willingness to persevere, open source may not be the right choice for you. Instead, consider building impactful projects that demonstrate your skills. The goal is to showcase proof of work, especially if you're pursuing a job.

Getting started with open-source projects can feel overwhelming, but with the right approach, it becomes manageable. Here’s a straightforward guide to help you contribute effectively:

## 1. **Choose an Organization**
- Explore organizations using resources like [gsocorganizations.dev](https://gsocorganizations.dev) or the GSoC Project/Organization list.
- Filter projects by the programming languages you know or want to contribute to.

## 2. **Use the Project**
- Use the software you plan to contribute to. This helps you gain essential context and familiarize yourself with terminology and concepts reflected in the codebase and software interface.

## 3. **Read the CONTRIBUTING.md**
- The `README.md` provides an overview of the project, while `CONTRIBUTING.md` outlines how you can contribute. 
- It is the **essential guide** for contributing. It includes steps for setting up the project locally, guidelines for issue assignment, testing procedures, and any separate issue trackers.

## 4. **Set Up the Project Locally**
- Follow the instructions in `CONTRIBUTING.md` to set up the project on your local environment.
- Build the project and familiarize yourself with the file structure, understanding where different components of the code are stored.

## 5. **Analyze Frequently Updated Files**
```
git log --pretty=format: --name-only | sort | uniq -c | sort -rg | head -10
```
- Identify files that are updated most frequently; these are likely critical components of the project. This aligns with the 80/20 rule—most impactful work happens in a minority of files.

## 6. **Understand the Repository Structure**
- Gain a high-level overview of the directory structure. For example:

```
  - Repository
  |
  |- directory1 # Contains X
  |- directory2 # Contains Y
  |- directory3 # Contains Z
```

- Dive into specific directories to inspect files and understand their purpose. Validate your assumptions with project maintainers if you're making significant changes.

## 7. **Focus on Your First PR**
- Choose a manageable issue and commit at least three weeks to it. Research thoroughly and familiarize yourself with the project context.
- Look up unfamiliar terms or concepts related to the issue in context of the organization you're contributing to.
- Check commit history and git blame when figuring out something in the codebase via the PR description that added the changes you're trying to wrap your head around. 
- Try to find file(s) where you might need to do the changes for the bug or the feature

## 8. **Use Debugging Tools**
- While debugging, simple print statements often work best for gaining insights quickly.
- Create and utilize your debugging hacks as needed.

## Conclusion
Contributing to open source takes effort and time, but it’s a rewarding experience. Start with a project aligned with your skills, understand its structure and culture, and focus on delivering a meaningful first PR. Be patient, persistent, and open to learning—the process becomes easier as you grow. 🚀
