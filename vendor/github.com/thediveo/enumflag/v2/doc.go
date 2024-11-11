/*
Package enumflag supplements the Golang CLI flag handling packages spf13/cobra
and spf13/pflag with enumeration flags.

For instance, users can specify enum flags as “--mode=foo” or “--mode=bar”,
where “foo” and “bar” are valid enumeration values. Other values which are not
part of the set of allowed enumeration values cannot be set and raise CLI flag
errors.

Application programmers then simply deal with enumeration values in form of
uints (or ints), liberated from parsing strings and validating enumeration
flags.
*/
package enumflag
