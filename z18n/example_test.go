package z18n_test

// Examples:
//
//   T("asd")                                          Just ID
//   T("asd", email)                                   With ID and params
//   T("asd|default msg %(email)", email)              With default message and params.
//   T("asd|default msg: %(email)", email, z18n.N(5))  Apply pluralisation.
