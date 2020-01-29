[config-functions]: https://github.com/kubernetes-sigs/kustomize/blob/master/cmd/config/docs/api-conventions/functions-spec.md
[mdrip]: https://github.com/monopole/mdrip

# Kubernetes Configuration Functions

The [config functions][config-functions] in this repo help you build a pipeline
for maintaining Kubernetes configuration files.

For more information see the tested README examples for each config function:
- [consul](/consul)
- [vault](/vault)
- [cfssl](/cfssl)

## Tests

The markdown files in this repo are used as tests by running the fenced code
blocks in a shell (thanks to [mdrip][mdrip]).

We target the latest changes in the `config` function orchestrator, so you will
need to install it from source to run the examples:

```sh
git clone https://github.com/kubernetes-sigs/kustomize.git
cd kustomize/cmd/config
go install
```
