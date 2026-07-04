<p style="text-align:center;" align="center"><a href="https://meshery.io"><picture>
 <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/meshery-logo-light-text-side.svg">
 <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/meshery-logo-dark-text-side.svg">
<img src="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/meshery-logo-dark-text-side.svg"
alt="Meshery Logo" width="70%" /></picture></a><br /><br /></p>
<p align="center">
<a href="https://hub.docker.com/r/meshery/meshery" alt="Docker pulls">
  <img src="https://img.shields.io/docker/pulls/meshery/meshery.svg" /></a>
<a href="https://github.com/issues?q=is%3Aopen%20is%3Aissue%20archived%3Afalse%20(org%3Ameshery%20OR%20org%3Aservice-mesh-performance%20OR%20org%3Aservice-mesh-patterns%20OR%20org%3Ameshery-extensions)%20label%3A%22help%20wanted%22%20" alt="GitHub issues by-label">
  <img src="https://img.shields.io/github/issues/meshery/meshery/help%20wanted.svg?color=informational" /></a>
<a href="https://github.com/meshery/meshery/blob/master/LICENSE" alt="LICENSE">
  <img src="https://img.shields.io/github/license/meshery/meshery?color=brightgreen" /></a>
<a href="https://artifacthub.io/packages/helm/meshery/meshery" alt="Artifact Hub Meshery">
  <img src="https://img.shields.io/endpoint?color=brightgreen&label=Helm%20Chart&style=plastic&url=https%3A%2F%2Fartifacthub.io%2Fbadge%2Frepository%2Fartifact-hub" /></a>  
<a href="https://goreportcard.com/report/github.com/meshery/meshery" alt="Go Report Card">
  <img src="https://goreportcard.com/badge/github.com/meshery/meshery" /></a>
<a href="https://github.com/meshery/meshery/actions" alt="Build Status">
  <img src="https://img.shields.io/github/actions/workflow/status/meshery/meshery/release-drafter.yml" /></a>
<a href="https://bestpractices.coreinfrastructure.org/projects/3564" alt="CLI Best Practices">
  <img src="https://bestpractices.coreinfrastructure.org/projects/3564/badge" /></a>
<a href="https://discuss.meshery.io" alt="Discuss Users">
  <img src="https://img.shields.io/discourse/users?label=discuss&logo=discourse&server=https%3A%2F%2Fdiscuss.meshery.io" /></a>
<a href="https://slack.meshery.io" alt="Join Slack">
  <img src="https://img.shields.io/badge/Slack-@meshery.svg?logo=slack" /></a>
<a href="https://twitter.com/intent/follow?screen_name=mesheryio" alt="Twitter Follow">
  <img src="https://img.shields.io/twitter/follow/mesheryio.svg?label=Follow+Meshery&style=social" /></a>
<a href="https://github.com/meshery/meshery/releases" alt="Meshery Downloads">
  <img src="https://img.shields.io/github/downloads/meshery/meshery/total" /></a>  
<a href="https://gurubase.io/g/meshery" alt="Meshery Guru">
  <img src="https://img.shields.io/badge/Gurubase-Ask%20Meshery%20Guru-006BFF" /></a>
<!-- <a href="https://app.fossa.com/projects/git%2Bgithub.com%2Fmeshery%2Fmeshery?ref=badge_shield" alt="License Scan Report">
  <img src="https://app.fossa.com/api/projects/git%2Bgithub.com%2Fmeshery%2Fmeshery.svg?type=shield"/></a>  
  -->
</p>

# MeshSync

MeshSync is Meshery's event-driven, continuous discovery and synchronization engine. It ensures that the configuration and operational state of Kubernetes (and any supported Meshery platform) are known to Meshery Server. When deployed into a Kubernetes cluster, MeshSync runs as a custom controller under the control of [Meshery Operator](https://docs.meshery.io/concepts/architecture/operator) and publishes resource changes over Meshery Broker (NATS).

MeshSync runs in one of two modes: **nats** (default - publishes Kubernetes resource events to NATS) and **file** (writes deduplicated cluster snapshots to disk, with no NATS or CRD dependency). Run `meshsync --help` for input parameters.

## Documentation

All MeshSync documentation lives in [Meshery Docs](https://docs.meshery.io) - the single source of truth. Please update the docs alongside code changes rather than duplicating details here.

📖 **Using & understanding MeshSync** - how it works and where it fits in Meshery's architecture:
- [MeshSync architecture](https://docs.meshery.io/concepts/architecture/meshsync)
- [Meshery architecture](https://docs.meshery.io/concepts/architecture) (logical and component architecture)

🛠️ **Contributing to MeshSync** - set up a development environment, build, test, and submit changes:
- [Contributing to MeshSync](https://docs.meshery.io/project/contributing/contributing-meshsync)

<div>&nbsp;</div>

## Join the Meshery community

<a name="contributing"></a><a name="community"></a>
Our projects are community-built and welcome collaboration. 👍 Be sure to see the <a href="https://meshery.io/community">Contributor Journey Map</a>. Jump into community <a href="https://slack.meshery.io">Slack</a> or <a href="https://discuss.meshery.io">discussion forum</a> to participate.

<p style="clear:both;">
<h3>Find your MeshMate</h3>

<p>MeshMates are experienced Meshery community members, who will help you learn your way around, discover live projects, and expand your community network. Connect with a MeshMate today!</p>

Learn more about the <a href="https://meshery.io/community#meshmates">MeshMates</a> program. <br />

</p>
<br /><br />
<div style="display: flex; justify-content: center; align-items:center;">
<div>
<a href="https://meshery.io/community"><img alt="Meshery Community" src="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/meshery/meshery-logo.svg" width="140px" style="margin-right:36px; margin-bottom:7px;" width="140px" align="left"/></a>
</div>
<div style="width:60%; padding-left: 16px; padding-right: 16px">
<p>
✔️ <em><strong>Join</strong></em> any or all of the weekly meetings on <a href="https://meshery.io/calendar">community calendar</a>.<br />
✔️ <em><strong>Watch</strong></em> community <a href="https://www.youtube.com/@mesheryio?sub_confirmation=1">meeting recordings</a>.<br />
✔️ <em><strong>Fill-in</strong></em> a <a href="https://meshery.io/newcomers">member form</a> and gain access to community resources.
<br />
✔️ <em><strong>Discuss</strong></em> in the <a href="https://discuss.meshery.io">community forum</a>.<br />
✔️ <em><strong>Explore more</strong></em> in the <a href="https://meshery.io/community#handbook">community handbook</a>.<br />
</p>
</div><br /><br />
<div>
<a href="https://slack.meshery.io">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/slack.svg"  width="110px" />
  <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/slack.svg" width="110px" />
  <img alt="Shows an illustrated light mode meshery logo in light color mode and a dark mode meshery logo dark color mode." src="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/slack.svg" width="110px" align="left" />
</picture>
</a>
</div>
</div>
<br /><br />
<p align="left">
&nbsp;&nbsp;&nbsp;&nbsp; <i>Not sure where to start?</i> Grab an open issue with the <a href="https://github.com/issues?q=is%3Aopen%20is%3Aissue%20archived%3Afalse%20(org%3Ameshery%20OR%20org%3Aservice-mesh-performance%20OR%20org%3Aservice-mesh-patterns%20OR%20org%3Ameshery-extensions)%20label%3A%22help%20wanted%22%20">help-wanted label</a>.
</p>
<br /><br />

<div>&nbsp;</div>

## Contributing

Please do! We're a warm and welcoming community of open source contributors. All types of contributions are welcome. Start with [Contributing to MeshSync](https://docs.meshery.io/project/contributing/contributing-meshsync) for a development-environment walkthrough, and see the general [Contributor Guides](https://docs.meshery.io/project/contributing) for a tour of the resources available to you and how to get started.

<div>&nbsp;</div>

### License

This repository and site are available as open-source under the terms of the [Apache 2.0 License](https://opensource.org/licenses/Apache-2.0).
