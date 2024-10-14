This version of the controller has nginx 1.25, 
which supports HTTP3, but it has not been added yet.

We add HTTP3, but the protocol is in experimental state, it is not suitable for use in production.
For example, the passtrough mechanism will not work, and there may be issues with authorization and proxy functions.