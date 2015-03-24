---
title: Contributor Guidelines
---


# Contributor Guidelines

## Getting Started

Maybe you already have a bugfix or enhancement in mind.  If not, there may be a
number of relatively approachable issues with the label
["good first bug"](https://github.com/mesosphere/mesos-dns/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+bug%22) that you can work on. We welcome contributions in code, documentation, and other resources. 

## Submitting Changes to Mesos-DNS

A GitHub pull request is the preferred way of submitting patch sets for bugfixes and enhancements of any kind. 

Any changes in the public API or behavior must be reflected in the documentation.

Pull requests should include appropriate additions to the unit test suite.

If the change is a bugfix, then the added tests must fail without the patch  as a safeguard against future regressions.

## Working with godep

To avoid complications with external packages, we use [`godep`](https://github.com/tools/godep). To use Mesos-DNS or make any changes to its own code, you will mostly use `make all` or `make build`. If you need to add a dependency to a package you started using, use `make savedeps`. If you need to restore package versions to what is specified by the branch of Mesos-DNS you are using, use `make restoredeps`. 

