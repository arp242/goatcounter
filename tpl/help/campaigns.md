Campaigns are tracked automatically based on URL query parameters:

- The campaign name is in the `utm_campaign` or `campaign` parameter.

- An optional source can be in the `utm_source`, `ref`, `src`, or `source`
  parameter (it will use the Referrer if this is missing).

There is no need to "create" campaigns; once it sees a campaign with a new name
it will be created automatically and shown in the Campaigns dashboard widget.

