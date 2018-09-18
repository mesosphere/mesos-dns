# Mesos-DNS [![Circle CI](https://circleci.com/gh/mesosphere/mesos-dns.svg?style=svg)](https://circleci.com/gh/mesosphere/mesos-dns) [![velocity](https://velocity.mesosphere.com/service/velocity/buildStatus/icon?job=public-mesos-dns-master)](https://velocity.mesosphere.com/service/velocity/job/public-mesos-dns-master/) [![Coverage Status](https://coveralls.io/repos/mesosphere/mesos-dns/badge.svg?branch=master&service=github)](https://coveralls.io/github/mesosphere/mesos-dns?branch=master) [![GoDoc](https://godoc.org/github.com/mesosphere/mesos-dns?status.svg)](https://godoc.org/github.com/mesosphere/mesos-dns) [![Gitter](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/mesosphere/mesos-dns?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

Mesos-DNS enables [DNS](https://en.wikipedia.org/wiki/Domain_Name_System)-based service discovery in [Apache Mesos](https://mesos.apache.org/) clusters.

![Architecture Diagram](https://mesosphere.github.io/mesos-dns/img/architecture.png)

## Compatibility

`mesos-N` tags mark the start of support for a specific Mesos version while
maintaining backwards compatibility with the previous major version.

## Installing

The official release binaries are available at [Github releases](https://github.com/mesosphere/mesos-dns/releases).

## Building

Building the **master** branch from source should always succeed but doesn't provide
the same stability and compatibility guarantees as releases.

All branches and pull requests are tested by [CircleCI](https://circleci.com/gh/mesosphere/mesos-dns), which also
outputs artifacts for Mac OS X, Windows, and Linux via cross-compilation.

You will need [Go](https://golang.org/) *1.6* or later to build the project.
All dependencies are tracked using `godep`.

```shell
# Install godep
$ go get github.com/tools/godep

# Save new dependencies
$ godep save ./...

# Build
$ go build ./...
```

### Building a release

1. Cut a branch.
2. Tag it with the relevant version, and push the tags along with the branch.
3. If the build doesn't trigger automatically, go to [CircleCI](https://circleci.com/gh/mesosphere/mesos-dns), find your branch, and trigger the build.

### Making a private build

1. Fork the repo on Github.
2. Customize that repo.
3. Add it to CircleCI. Please note that CircleCI allows for private repositories to be kept, and built in private.
4. Go to the build steps.

#### Releasing

1. Download the artifacts from CircleCI.
2. Cut a release based on the tag on Github.
3. Upload the artifacts back to Github. Ensure you upload all the artifacts, including the `.asc` files.

#### Code signing

This repo uses code signing. There is an armored, encrypted GPG key in the repo in [build/private.key](build/private.key). This file includes the Mesos-DNS GPG signing key. The passphrase for the key is stored in Circle-CI's environment. This makes it fairly difficult to leak both components without detectable maliciousness.

There are only very few users with access to the private key, and they also have access to a revocation certificate in case the private key leaks.

## Testing

```shell
go test -race ./...
```

## Documentation

The detailed documentation on how to configure, operate and use Mesos-DNS
under different scenarios and environments is available at the project's [home page](https://mesosphere.github.io/mesos-dns/).

## Contributing

Contributions are welcome. Please refer to [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Contact

For any discussion that isn't well suited for Github [issues](https://github.com/mesosphere/mesos-dns/issues),
please use our [mailing list](https://groups.google.com/forum/#!forum/mesos-dns) or our public [chat room](https://gitter.im/mesosphere/mesos-dns).

## License

This project is licensed under [Apache License 2.0](LICENSE).
