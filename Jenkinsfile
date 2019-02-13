#!groovy

@Library('sec_ci_libs@v2-latest') _

env.PROJECT_DIR = '/go/src/github.com/mesosphere/mesos-dns'

ansiColor('xterm') {

  node("golang111") {
    properties([
      parameters([
        string(name: "SLACK_CREDENTIAL_ID", defaultValue: "8f0e60ea-1d59-4885-965e-3003bc0a9054"),
        string(name: "SLACK_CHANNEL", defaultValue: "#mesos-dns"),
        string(name: "ALERTS_FOR_BRANCHES", defaultValue: "master")
      ])
    ])

    stage("Verify author") {
      def alerts_for_branches = params.ALERTS_FOR_BRANCHES.tokenize(",") as String[]
      user_is_authorized(alerts_for_branches, params.SLACK_CREDENTIAL_ID, params.SLACK_CHANNEL)
    }

    deleteDir()

    stage("Build and Test") {
      dir (env.PROJECT_DIR) {
        checkout scm

        def packageSHA = getPackageSHA()
        def packageVersion = getPackageVersion()

        withEnv(["PACKAGE_SHA=${packageSHA}","PACKAGE_VERSION=${packageVersion}"]) {
          timeout(30) {
            sh("./ci.sh")
          }
        }
      }

      junit("test_results/junit/alltests.xml")
      archiveArtifacts(artifacts: "target/*")
    }
  }
}

def getPackageSHA() {
  def isPullRequest = (env.CHANGE_ID != null)

  if (isPullRequest) {
    def parents = sh(
          returnStdout: true,
          script: "git log --pretty=%P -n 1 HEAD").trim().split()

    if (parents.size() != 1) {
      // Non fast-forward case.
      return parents[0]
    }
  }

  return sh(
      returnStdout: true,
      script: "git rev-parse HEAD").trim()
}

def getPackageVersion() {
  def packageSHA = getPackageSHA()

  return sh(
      returnStdout: true,
      script: "git describe --exact-match ${packageSHA} 2>/dev/null || echo ${packageSHA}").trim()
}
