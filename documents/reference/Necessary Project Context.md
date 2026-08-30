# Locus / Locus Link — Necessary Project Context

Read this as project context, not as an architecture specification.

Previous Locus documents contain concrete design decisions. Those decisions are provisional and should be challenged when they introduce unnecessary complexity. Do not infer that concepts such as graph, registry, route, scope, provider, YAML, SQLite, CLI, daemon, or any particular programming language are requirements merely because they appeared in an earlier draft.

The purpose of this context is to explain why Locus exists, what real problems led to it, and what constraints a useful solution must respect.

---

## Where the project came from

Locus did not begin as an attempt to design a general infrastructure platform.

It grew out of repeated practical work on deployment, remote debugging, CI/CD, machine configuration, and agent-assisted development across heterogeneous machines.

The recurring environment includes combinations of:

- Windows development machines;
- Windows Server, including legacy systems;
- Linux machines and servers;
- public cloud/bastion machines;
- customer/internal-network machines;
- machines that cannot accept inbound connections;
- NAT, firewalls and restricted ports;
- SSH and WinRM;
- FRP;
- Salt;
- Gitea and CI/CD workers;
- Docker;
- DSC;
- Nix;
- Headscale/Tailscale/NetBird experiments;
- remote-management systems such as MeshCentral;
- agent-oriented systems such as Overseer;
- manual procedures that exist because the environment is irregular.

These are not hypothetical requirements. They come from infrastructure we have actually operated or attempted to operate.

One representative topology already used in practice is roughly:

local/dev infrastructure
→ public FRP/bastion infrastructure
→ machines inside otherwise difficult-to-reach networks

There have also been CI/CD arrangements where a worker inside another network reaches Gitea through FRP.

Salt has been deployed and tested against real machines. We have used it for remote commands and filesystem access.

Headscale/Tailscale were investigated as a cleaner network solution, but legacy Windows environments and deployment/network constraints demonstrated that we cannot assume every machine can join one modern overlay network cleanly.

NetBird was also attempted and exposed its own deployment/environment friction.

This history matters: heterogeneity is not an edge case to eliminate before solving the problem. It is the normal operating condition.

---

## How our understanding of the problem changed

The investigation originally looked much more like:

"How do we make CI/CD and deployment across these machines easier?"

That led to Docker, Ansible, DSC, Salt and related tools.

A second question then became dominant:

"How does the development machine or coding agent reliably reach the target machine at all?"

That led to Headscale/Tailscale, NetBird, FRP, SSH, WinRM, bastions and Salt's connectivity model.

Once connectivity worked, another distinction became obvious:

reachability is not execution;
execution is not configuration management;
configuration management is not deployment;
deployment is not application runtime.

For example:

- FRP can make something reachable but is not itself the remote shell we ultimately want.
- SSH can provide a shell but does not describe the desired state of the machine.
- Salt can remotely execute and manage machines but does not make every operational relationship an interactive SSH-like connection.
- DSC describes and enforces system state but does not solve arbitrary network topology.
- Docker addresses application/runtime packaging.
- Nix addresses reproducibility and environment construction.
- CI/CD systems orchestrate delivery workflows.
- RMM systems solve yet another overlapping set of problems.

We therefore stopped looking for one tool that should replace all of the others.

The important observation became:

The underlying mechanisms are often already good enough.

What repeatedly disappears between projects, machines, humans and agent sessions is the knowledge of how those mechanisms apply in this particular environment.

---

## The recurring operational failure

A human who has worked on an environment for a while gradually knows facts such as:

- which machines matter;
- what role they play;
- where they are;
- which machine can reach which other machine;
- which access mechanism actually works;
- which bastion or tunnel is involved;
- which tool is installed where;
- what constraints a legacy machine has;
- whether Salt, SSH, FRP, WinRM or some other mechanism is appropriate;
- which strange workaround was discovered last time;
- whether a path is currently likely to work;
- what must happen before another operation becomes possible.

A new coding agent usually knows none of this.

It reconstructs the situation by reading some combination of:

- README files;
- AGENTS.md;
- infrastructure configs;
- FRP configs;
- Salt configs;
- CI scripts;
- old notes;
- old conversations;
- shell history;
- repository files;
- explanations from a human;
- trial and error.

This is expensive even when every individual tool is already installed and functioning.

Worse, the result of that investigation frequently lives only in the current human/agent session and is rediscovered later.

This repeated rediscovery is the central pain that motivated Locus.

---

## The agent-first perspective

The project is specifically influenced by coding agents becoming active operators of infrastructure.

For a human, "there is an FRP configuration somewhere and the deployment engineer knows how it works" may be tolerable.

For an agent, operational knowledge must become available at the moment it needs to act.

The desired experience is conceptually:

the agent encounters a project/task
→ understands the relevant operational environment quickly
→ knows which existing mechanisms are available
→ uses those mechanisms directly
→ does not reconstruct the entire infrastructure history first

The important phrase is **agent fast path**.

The goal is not necessarily to reduce every operation to one API call.

The goal is to greatly reduce the amount of environment-specific archaeology required before an agent can perform the operation correctly.

---

## Generic knowledge and situated knowledge are different

One distinction from the previous discussions should be preserved even if every concrete design is discarded.

Generic procedural knowledge answers questions such as:

- how SSH ProxyJump works;
- how to use FRP;
- how to invoke Salt;
- how DSC works;
- how to restart a Docker container;
- how to deploy through Gitea.

This knowledge belongs naturally in documentation, Skills, model knowledge, manuals, etc.

But it does not answer:

- which host is the production host for this project;
- which FRP server this environment uses;
- which unusual path currently reaches a customer machine;
- whether this old Windows machine supports a particular tool;
- which mechanism should be tried from the current development machine;
- what was discovered during the previous incident.

That is situated, instance-specific and often changing knowledge.

A useful heuristic from the earlier discussion was:

    Action ≈ generic know-how + situated context

This is not an API or data-model requirement. It is a distinction about where the unresolved problem lies.

A Skill repository alone therefore does not obviously solve the problem.

Conversely, Locus should not become another repository of generic tutorials that models already know how to follow.

---

## Why existing remote-management systems do not automatically end the project

We investigated RMM and agent-first fleet systems precisely because they are very close to part of the desired experience.

MeshCentral demonstrates that a central system plus installed agents can provide remote desktop, terminal and files.

Overseer goes even closer to our use case: devices connect outward to a hub and the system exposes terminals, files, coding-agent sessions, CLI operations and MCP tools that other agents can use.

This is important prior art, not something to duplicate casually.

If we were willing and able to make every relevant machine an Overseer/MeshCentral-style managed node, much of the remote-control problem could simply be delegated to such a system.

But our actual infrastructure already contains several independent mechanisms and difficult/legacy environments.

Some paths work through Salt.
Some work through SSH.
Some require FRP.
Some exist inside CI/CD.
Some may use an RMM.
Some are local operations.
Some may remain partially manual.

Requiring the entire world to first converge onto one new Locus agent/runtime would change the problem into building another RMM.

That is not currently justified.

Existing systems should therefore be treated both as reusable infrastructure and as evidence against duplicating mature functionality.

---

## Capability was a useful perspective, but should not become dogma

During the discussion we noticed that an agent often cares less about product categories than about what the current environment allows it to do.

A network path, SSH access, a Salt minion, an installed program, a CI pipeline, a local tool, or even a documented human-assisted procedure may all contribute to what is operationally possible.

This "capability perspective" was useful because it broke the assumption that all useful relationships must belong to one technology.

However, this does not mean Locus must now construct a universal capability ontology.

The perspective is valuable; a giant abstraction hierarchy is not automatically valuable.

Use abstractions only when repeated real cases force them.

---

## Skill versus runtime was another important realization

A static Skill can describe what to do.

Real operation additionally depends on things that exist only in a concrete situation:

- actual machines;
- actual identities;
- installed tools;
- credentials/permissions;
- topology;
- current connectivity;
- current state;
- project-specific bindings;
- environmental constraints;
- observations made recently.

This led us to the broader observation that a useful agent runtime can never consist entirely of generic Skills.

At some point, live state and concrete environment bindings must enter the system.

There is therefore probably some irreducible runtime/contextual core in any system that tries to operate the real world.

But this insight should NOT be used as permission to build a general-purpose agent runtime now.

One of the recurring mistakes in this exploration has been to generalize a local problem until it resembles an operating system or universal agent platform.

When that happens, narrow the problem again.

---

## Do not prematurely create a universal host abstraction

We previously considered interfaces conceptually resembling:

    host.exec(...)
    host.files(...)
    host.deploy(...)
    host.service(...)

and became skeptical of using such an abstraction as the center of Locus.

The existing systems have meaningful differences.

Salt, SSH, FRP, DSC, Docker and native host tools expose different semantics and different operational context.

Modern coding agents also already understand many of these tools directly.

Wrapping everything too early can therefore:

- discard useful native context;
- create a lowest-common-denominator interface;
- duplicate existing APIs;
- require Locus to continuously emulate unrelated tools;
- make the agent less able to reason about what is really happening.

The broader lesson is:

Do not hide heterogeneity merely to make the architecture look uniform.

Uniformity is useful only when it removes more complexity than it creates.

---

## Multiple projects and shared environments are part of the real problem

The developer machine may contain many projects.

Some operational facts belong specifically to one project.

Other facts describe shared infrastructure that remains true regardless of which project is open.

Examples of shared facts can include:

- a bastion machine;
- a Salt master;
- a customer environment;
- a shared deployment machine;
- an existing network path.

Examples of project-specific facts can include:

- which environment the project deploys to;
- which service belongs to the project;
- which existing infrastructure the project uses;
- project-specific operational conventions.

Eventually humans may also want a unified view across projects and environments.

This distinction is required by the problem domain.

How it should be represented is still an implementation/design question and should not be assumed from previous drafts.

---

## Human knowledge remains part of the system

The intended world is not one in which autonomous agents magically discover every relationship.

A common and valuable lifecycle is:

a human or agent solves an unusual infrastructure problem once
→ the useful result becomes persistent shared knowledge
→ later humans and agents reuse it

Humans must remain able to inspect, correct and explain operational knowledge.

The long-term system may support centralized monitoring or management, but human-facing Web UI, server architecture, synchronization and governance are not prerequisites for proving the core value.

---

## Locus is not justified merely because its abstractions are interesting

We have repeatedly reached broad and intellectually attractive ideas:

- capability systems;
- graph-like representations;
- runtime composition;
- action graphs;
- dependency resolution;
- agent runtimes;
- executable infrastructure models;
- generalized orchestration.

Some of these may eventually be correct.

They are not sufficient reasons to build them.

The project should remain anchored in reducing real operational pain.

This is especially important because many neighboring domains already contain mature systems:

- networking / overlays;
- reverse proxies;
- SSH;
- remote execution;
- RMM;
- configuration management;
- desired-state systems;
- CI/CD;
- workflow engines;
- container runtimes;
- package/environment managers;
- CMDB/inventory systems.

Locus should not rediscover one of those systems under new terminology.

If an existing system already solves a problem cleanly, reuse it.

---

## What we currently believe Locus is trying to achieve

At the highest level, the current hypothesis is:

There is useful operational knowledge between
"generic instructions"
and
"the underlying infrastructure mechanisms".

It is knowledge specific to a concrete project/environment and to the current operational situation.

Making that knowledge persistent and easily consumable by agents may allow a newly arrived agent to make effective use of heterogeneous existing infrastructure without rediscovering its topology and history.

This is the hypothesis to test.

"Locus Link" is the current name for the first narrow attempt to test it, centered on the operational relationships required to reach and use resources.

Do not infer more architecture from the name.

---

## What Locus Link should not have to prove

It does not need to prove that:

- graphs are a superior representation;
- everything can be modeled as a capability;
- infrastructure should use one universal API;
- every operation should become executable through Locus;
- Locus should control the machines itself;
- agents can automatically discover all topology;
- Locus should replace Skills;
- Locus should replace CI/CD;
- Locus should replace Salt, SSH, FRP, DSC, Docker, Nix, RMM, VPN/overlay networks, or existing configuration systems;
- Locus should become a general agent runtime;
- Locus must become a large product.

Any previous design that implicitly requires one of these claims should be questioned.

---

## Value test

The relevant comparison is not:

"Can Locus represent this information?"

Almost anything can represent information.

The comparison is:

"Does Locus materially reduce the amount of project/environment-specific rediscovery required before an agent can perform useful work?"

A good test involves real environments rather than synthetic examples.

For example, give an agent a concrete task involving an existing nontrivial infrastructure path without first feeding it old conversations, infrastructure notes, FRP/Salt configuration explanations and human tribal knowledge.

Then ask whether Locus allows the agent to understand and reuse the path substantially faster and more reliably.

This should work across more than one real project/environment.

If the answer is no—if Locus merely moves information from README/AGENTS.md/Skills into another representation and prints it back—then an independent Locus system is probably not justified.

In that case it should collapse into something simpler, such as improved documentation, a Skill, a lightweight registry, or another existing tool.

Failure of the hypothesis is an acceptable outcome.

---

## Attitude for the next design pass

When reviewing or modifying the existing Locus Link design:

- treat the problem and real scenarios as authoritative;
- treat previous architecture as provisional;
- aggressively identify abstractions that exist only because an earlier abstraction required them;
- distinguish domain complexity from self-created framework complexity;
- prefer existing mechanisms over reimplementation;
- preserve native tool semantics when they are useful;
- demand a real use case for every new concept;
- avoid solving hypothetical organizational-scale problems before they appear;
- avoid designing future execution/runtime machinery merely because it may someday be useful;
- keep future evolution possible without paying its complexity cost now.

The project has already gone through several rounds of expansion and re-narrowing.

The next useful step is not to make the conceptual model more complete.

It is to determine the smallest thing that genuinely eliminates a recurring piece of operational rediscovery for agents.