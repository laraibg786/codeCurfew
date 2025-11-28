# codeCurfew format

This configuration format allows defining curfew times for code commits on a per-day basis.
Times can be specified in UTC to avoid the timezone confusion.

> [!NOTE]
> Conversion can be done using the [dateful.com](https://dateful.com/convert/utc)

### File Structure:

- One and only one line per day. All days should be defined to leave out the ambiguity.
- Extra definition of the weekdays override the older definitions.
- Lines starting with `#`, `//` are comments and ignored along with the blank lines.
- Format per line: `<Day Name> <Start Time> - <End Time>`
- `-` is used to separate the start and end time.
- Lines starting with ! represent the allow list.

### Fields:

| Field           | Description                                                              |
|-----------------|--------------------------------------------------------------------------|
|<Day Name>       | Full weekday names (case-insensitive)                                    |
|<Start/End Time> | Time in 24H format in UTC. `*` can be used to define the remaining time. |

> [!IMPORTANT]
> First value in time is start time and the second time is the end time.

## Examples:

```
# Disallow on Friday after 04:30 PM UTC
! Friday 1630 - *

# Allow on Monday after 10:00 AM
Monday 1000 - *

# Disallow on Saturday and Sunday
! Saturday * - *
! Sunday * - *

# Allow Tuesday - Thursday till 05:00 PM
Tuesday * - 1700
Wednesday * - 1700
Thursday * - 1700
Friday * - 1700
```

## Template:

You can copy the following (default) template and customize to your liking

```
Monday 1000 - *
Tuesday * - *
Wednesday * - *
Thursday * - *
! Friday 1400 - *
! Saturday * - *
! Sunday * - *
```
