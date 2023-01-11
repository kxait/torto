# torto
command runner similar to https://yake.amsdard.io/

## usage
- torto hello_world
    runs the `hello_world` target
- torto demo VAR1=value1
    runs `demo` target with `VAR1` set to `value`
- torto run -- uname -i
    runs the `run` target with `CMD` set to `uname -i`
- torto -d -f demo -- uname -i
    runs in debug and force mode
