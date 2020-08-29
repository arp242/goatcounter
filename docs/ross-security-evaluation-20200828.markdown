Radically Open Security Report
==============================

1 Introduction
--------------

As part of the NLnet projects your project GoatCounter has received a basic
quick security evaluation from RadicallyOpen Security. The goal of the review is
to provide advice and input to consider in the further development of your
project. The selected project gets 2 person days from ROS for a quick security
evaluation (one day at the start and one day at a later stadium or at the end of
the project).

2 Security Evaluation For GoatCounter
-------------------------------------

### Tasks Performed

- Scanned the repository at https://github.com/zgoat/goatcounter (v1.4)
- Performed automated web security scanning of a self hosted version of GoatCounter (v1.4)
- Performed automated static code analysis of the Golang code (v1.4)
- Manual validation of security issues discovered in automated security scan

### Security Considerations

- Input validation and outout encoding is not properly enforced throughout the
  application. This may lead to vulnerabilities like cross site scripting, HTML
  injection etc.

  For example: `/period-start=4020-<script>alert(0)</script>-27`

- Anti CSRF token validation missing on server side. Though, the samesite cookie
  flag will save the application from CSRF attacks.

- The endpoint /ip accepts any value in the X-Forwarded-For header:
  `X-Forwarded-For: localhost`.

- Weak cryptographic/hashing algorithm SHA1 is being used in `tplfunc.go` header.

- Subprocess launched with variable in `admin.go`:

      drill, err := exec.Command("drill", "-x", ip).CombinedOutput()

  Reference: https://cwe.mitre.org/data/definitions/78.html

### Recommendations

- It is recommended to implement strong input validation for all application
  endpoints. Especially, a context based output encoding should be done on the
  user's input when it becomes a part of HTML response. Please refer toOWASP's
  Cross Site Scripting Prevention Cheat Sheet here:
  https://cheatsheetseries.owasp.org/cheatsheets/Cross_Site_Scripting_Prevention_Cheat_Sheet.html

- The anti CSRF token used in several requests, should also be validated at the
  application end, to ensure that no tampering of the request has been done and
  CSRF attacks do not take place.

- Trusting all values provided in the X-Forwarded-For header by user, might lead
  to infrastructure security control bypass in some cases. It is not recommended
  to simply trust this value in the request.

- It is not recommended to use SHA-1 (Secure Hash Algorithm 1), a weak
  cryptographic hash function. SHA1 is known to have collision issues, hence it
  might be possible to create same hash from a two different inputs. It is
  recommended to only use strong hashing algorithms, for eg SHA256, SHA512.

- Assume all input is malicious. Use an "accept known good" input validation
  strategy, i.e., use a list of acceptable inputs that strictly conform to
  specifications. Reject any input that does not strictly conform to
  specifications, or transform it into something that does.

Implemented fixes
=================

- The flash messages allowed HTML for just one link in one message; this has now
  been changed to static text. Note the injected code didn't actually get
  executed because the Content-Security-Policy doesn't allow `unsafe-inline`.

  https://github.com/zgoat/zhttp/commit/2ed780c084b49bc8fca673d8df9b9c46710633be
  https://github.com/zgoat/goatcounter/commit/11ee784281d74007736272d59cf15d839fbf9af3

- The CSRF tokens were already implemented, but the application didn't actually
  abort if they're missing or wrong >_< The SameSite flag on the cookie already
  prevented CSRF attacks, but doesn't hurt to add an additional protection, so
  now the app aborts if it's missing.

  https://github.com/zgoat/zhttp/commit/c645d9182aace886d49d1364cb37ff13c12b67a7

- The `X-Forwarded-For` header now allows only valid IP addresses, and filters
  out non-public addresses such as localhost, RFC1918, etc.

  https://github.com/zgoat/zhttp/commit/3eb3df60ddebfbe954e4b1521ece0628901f259b

- The SHA1 isn't used for crypto purposes, and not a problem in this use case.
  GoatCounter is added on the goatcounter backend as well so I have some insight
  in which features people are using, and the hash is used to "anonymize" the
  domain codes so I don't see who specifically is clicking on what (this is a
  really weak anonymisation, just so I don't have this information when looking
  at the interface).

- The `drill` command takes the IP from the remote address or `X-Forwarded-For`
  header; since this is now strictly limited to valid IP addresses this should
  be fine (it's also not a shell command, so it shouldn't really be a problem,
  unless there's a vulnerability in `drill` or `whois`).
