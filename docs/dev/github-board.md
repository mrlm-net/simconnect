# GitHub Projects v2 board

Not published on the site (the site reads `docs/*.md` only, not subfolders).

**Project**: SimConnect GoLang SDK
**URL**: https://github.com/orgs/mrlm-net/projects/7
**Project number**: 7
**Owner**: mrlm-net
**Project ID**: PVT_kwDOBxaH0c4A9IjR

## Custom fields

| Field | Field ID | Options |
|-------|----------|---------|
| Status | `PVTSSF_lADOBxaH0c4A9IjRzgw7UX8` | Backlog=`f75ad846`, Ready=`61e4505c`, In progress=`47fc9ee4`, In review=`df73e18b`, Done=`98236657` |
| Priority | `PVTSSF_lADOBxaH0c4A9IjRzgw7UcQ` | P0=`79628723`, P1=`0a877460`, P2=`da944a9c` |
| Size | `PVTSSF_lADOBxaH0c4A9IjRzgw7UcU` | XS=`6c6483d2`, S=`f784b110`, M=`7515a9f1`, L=`817d0097`, XL=`db339eb2` |

## Board update commands

```bash
# Add issue to board
gh project item-add 7 --owner mrlm-net --url <ISSUE_URL>

# Get item ID
gh project item-list 7 --owner mrlm-net --format json -L 100

# Update field
gh project item-edit --project-id PVT_kwDOBxaH0c4A9IjR --id <ITEM_ID> --field-id <FIELD_ID> --single-select-option-id <OPTION_ID>
```
