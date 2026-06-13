Warning!

During move provider to standalone module, please move logic from `pkg/config/prepare_yaml.go`

We do not implement another preparator for yamls in cloud provider, because it needs huge refactoring.

And we do not use MetaConfig preparator because we do not have information about cloud.

Otherwise, we have cyclic deps.
