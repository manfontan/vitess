/*
Copyright 2019 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlparser

import (
	"bufio"
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	validSQL = []struct {
		input      string
		output     string
		partialDDL bool
	}{{
		input:  "select 1",
		output: "select 1 from dual",
	}, {
		input: "select 1 from t",
	}, {
		input:  "select * from information_schema.columns",
		output: "select * from information_schema.`columns`",
	}, {
		input:  "select * from information_schema.processlist",
		output: "select * from information_schema.`processlist`",
	}, {
		input: "select .1 from t",
	}, {
		input: "select 1.2e1 from t",
	}, {
		input: "select 1.2e+1 from t",
	}, {
		input: "select 1.2e-1 from t",
	}, {
		input: "select 08.3 from t",
	}, {
		input: "select -1 from t where b = -2",
	}, {
		input:  "select - -1 from t",
		output: "select 1 from t",
	}, {
		input: "select a from t",
	}, {
		input: "select $ from t",
	}, {
		input: "select a.b as a$b from $test$",
	}, {
		input:  "select 1 from t // aa\n",
		output: "select 1 from t",
	}, {
		input:  "select 1 from t -- aa\n",
		output: "select 1 from t",
	}, {
		input:  "select 1 from t # aa\n",
		output: "select 1 from t",
	}, {
		input:  "select 1 --aa\nfrom t",
		output: "select 1 from t",
	}, {
		input:  "select 1 #aa\nfrom t",
		output: "select 1 from t",
	}, {
		input: "select /* simplest */ 1 from t",
	}, {
		input: "select /* double star **/ 1 from t",
	}, {
		input: "select /* double */ /* comment */ 1 from t",
	}, {
		input: "select /* back-quote keyword */ `By` from t",
	}, {
		input: "select /* back-quote num */ `2a` from t",
	}, {
		input: "select /* back-quote . */ `a.b` from t",
	}, {
		input: "select /* back-quote back-quote */ `a``b` from t",
	}, {
		input:  "select /* back-quote unnecessary */ 1 from `t`",
		output: "select /* back-quote unnecessary */ 1 from t",
	}, {
		input:  "select /* back-quote idnum */ 1 from `a1`",
		output: "select /* back-quote idnum */ 1 from a1",
	}, {
		input: "select /* @ */ @@a from b",
	}, {
		input: "select /* \\0 */ '\\0' from a",
	}, {
		input:  "select 1 /* drop this comment */ from t",
		output: "select 1 from t",
	}, {
		input: "select /* union */ 1 from t union select 1 from t",
	}, {
		input: "select /* double union */ 1 from t union select 1 from t union select 1 from t",
	}, {
		input: "select /* union all */ 1 from t union all select 1 from t",
	}, {
		input:  "select /* union distinct */ 1 from t union distinct select 1 from t",
		output: "select /* union distinct */ 1 from t union select 1 from t",
	}, {
		input:  "(select /* union parenthesized select */ 1 from t order by a) union select 1 from t",
		output: "(select /* union parenthesized select */ 1 from t order by a asc) union select 1 from t",
	}, {
		input: "select /* union parenthesized select 2 */ 1 from t union (select 1 from t)",
	}, {
		input:  "select /* union order by */ 1 from t union select 1 from t order by a",
		output: "select /* union order by */ 1 from t union select 1 from t order by a asc",
	}, {
		input:  "select /* union order by limit lock */ 1 from t union select 1 from t order by a limit 1 for update",
		output: "select /* union order by limit lock */ 1 from t union select 1 from t order by a asc limit 1 for update",
	}, {
		input: "select /* union with limit on lhs */ 1 from t limit 1 union select 1 from t",
	}, {
		input:  "(select id, a from t order by id limit 1) union (select id, b as a from s order by id limit 1) order by a limit 1",
		output: "(select id, a from t order by id asc limit 1) union (select id, b as a from s order by id asc limit 1) order by a asc limit 1",
	}, {
		input: "select a from (select 1 as a from tbl1 union select 2 from tbl2) as t",
	}, {
		input: "select * from t1 join (select * from t2 union select * from t3) as t",
	}, {
		// Ensure this doesn't generate: ""select * from t1 join t2 on a = b join t3 on a = b".
		input: "select * from t1 join t2 on a = b join t3",
	}, {
		input: "select * from t1 where col in (select 1 from dual union select 2 from dual)",
	}, {
		input: "select * from t1 where exists (select a from t2 union select b from t3)",
	}, {
		input: "select 1 from dual union select 2 from dual union all select 3 from dual union select 4 from dual union all select 5 from dual",
	}, {
		input: "(select 1 from dual) order by 1 asc limit 2",
	}, {
		input: "(select 1 from dual order by 1 desc) order by 1 asc limit 2",
	}, {
		input: "(select 1 from dual)",
	}, {
		input: "((select 1 from dual))",
	}, {
		input: "select 1 from (select 1 from dual) as t",
	}, {
		input: "select 1 from (select 1 from dual union select 2 from dual) as t",
	}, {
		input: "select 1 from ((select 1 from dual) union select 2 from dual) as t",
	}, {
		input: "select /* distinct */ distinct 1 from t",
	}, {
		input: "select /* straight_join */ straight_join 1 from t",
	}, {
		input: "select /* for update */ 1 from t for update",
	}, {
		input: "select /* lock in share mode */ 1 from t lock in share mode",
	}, {
		input: "select /* select list */ 1, 2 from t",
	}, {
		input: "select /* * */ * from t",
	}, {
		input: "select /* a.* */ a.* from t",
	}, {
		input: "select /* a.b.* */ a.b.* from t",
	}, {
		input:  "select /* column alias */ a b from t",
		output: "select /* column alias */ a as b from t",
	}, {
		input: "select /* column alias with as */ a as b from t",
	}, {
		input: "select /* keyword column alias */ a as `By` from t",
	}, {
		input:  "select /* column alias as string */ a as \"b\" from t",
		output: "select /* column alias as string */ a as b from t",
	}, {
		input:  "select /* column alias as string without as */ a \"b\" from t",
		output: "select /* column alias as string without as */ a as b from t",
	}, {
		input:  "select /* column alias with non_reserved keyword */ a as auto_increment from t",
		output: "select /* column alias with non_reserved keyword */ a as `auto_increment` from t",
	}, {
		input: "select /* a.* */ a.* from t",
	}, {
		input:  "select next value for t",
		output: "select next 1 values from t",
	}, {
		input:  "select next value from t",
		output: "select next 1 values from t",
	}, {
		input: "select next 10 values from t",
	}, {
		input: "select next :a values from t",
	}, {
		input: "select /* `By`.* */ `By`.* from t",
	}, {
		input: "select /* select with bool expr */ a = b from t",
	}, {
		input: "select /* case_when */ case when a = b then c end from t",
	}, {
		input: "select /* case_when_else */ case when a = b then c else d end from t",
	}, {
		input: "select /* case_when_when_else */ case when a = b then c when b = d then d else d end from t",
	}, {
		input: "select /* case */ case aa when a = b then c end from t",
	}, {
		input: "select /* parenthesis */ 1 from (t)",
	}, {
		input: "select /* parenthesis multi-table */ 1 from (t1, t2)",
	}, {
		input: "select /* table list */ 1 from t1, t2",
	}, {
		input: "select /* parenthessis in table list 1 */ 1 from (t1), t2",
	}, {
		input: "select /* parenthessis in table list 2 */ 1 from t1, (t2)",
	}, {
		input: "select /* use */ 1 from t1 use index (a) where b = 1",
	}, {
		input: "select /* use */ 1 from t1 use index () where b = 1",
	}, {
		input: "select /* keyword index */ 1 from t1 use index (`By`) where b = 1",
	}, {
		input: "select /* ignore */ 1 from t1 as t2 ignore index (a), t3 use index (b) where b = 1",
	}, {
		input: "select /* use */ 1 from t1 as t2 use index (a), t3 use index (b) where b = 1",
	}, {
		input: "select /* force */ 1 from t1 as t2 force index (a), t3 force index (b) where b = 1",
	}, {
		input:  "select /* table alias */ 1 from t t1",
		output: "select /* table alias */ 1 from t as t1",
	}, {
		input: "select /* table alias with as */ 1 from t as t1",
	}, {
		input:  "select /* string table alias */ 1 from t as 't1'",
		output: "select /* string table alias */ 1 from t as t1",
	}, {
		input:  "select /* string table alias without as */ 1 from t 't1'",
		output: "select /* string table alias without as */ 1 from t as t1",
	}, {
		input: "select /* keyword table alias */ 1 from t as `By`",
	}, {
		input: "select /* join */ 1 from t1 join t2",
	}, {
		input: "select /* join on */ 1 from t1 join t2 on a = b",
	}, {
		input: "select /* join on */ 1 from t1 join t2 using (a)",
	}, {
		input:  "select /* inner join */ 1 from t1 inner join t2",
		output: "select /* inner join */ 1 from t1 join t2",
	}, {
		input:  "select /* cross join */ 1 from t1 cross join t2",
		output: "select /* cross join */ 1 from t1 join t2",
	}, {
		input: "select /* straight_join */ 1 from t1 straight_join t2",
	}, {
		input: "select /* straight_join on */ 1 from t1 straight_join t2 on a = b",
	}, {
		input: "select /* left join */ 1 from t1 left join t2 on a = b",
	}, {
		input: "select /* left join */ 1 from t1 left join t2 using (a)",
	}, {
		input:  "select /* left outer join */ 1 from t1 left outer join t2 on a = b",
		output: "select /* left outer join */ 1 from t1 left join t2 on a = b",
	}, {
		input:  "select /* left outer join */ 1 from t1 left outer join t2 using (a)",
		output: "select /* left outer join */ 1 from t1 left join t2 using (a)",
	}, {
		input: "select /* right join */ 1 from t1 right join t2 on a = b",
	}, {
		input: "select /* right join */ 1 from t1 right join t2 using (a)",
	}, {
		input:  "select /* right outer join */ 1 from t1 right outer join t2 on a = b",
		output: "select /* right outer join */ 1 from t1 right join t2 on a = b",
	}, {
		input:  "select /* right outer join */ 1 from t1 right outer join t2 using (a)",
		output: "select /* right outer join */ 1 from t1 right join t2 using (a)",
	}, {
		input: "select /* natural join */ 1 from t1 natural join t2",
	}, {
		input: "select /* natural left join */ 1 from t1 natural left join t2",
	}, {
		input:  "select /* natural left outer join */ 1 from t1 natural left join t2",
		output: "select /* natural left outer join */ 1 from t1 natural left join t2",
	}, {
		input: "select /* natural right join */ 1 from t1 natural right join t2",
	}, {
		input:  "select /* natural right outer join */ 1 from t1 natural right join t2",
		output: "select /* natural right outer join */ 1 from t1 natural right join t2",
	}, {
		input: "select /* join on */ 1 from t1 join t2 on a = b",
	}, {
		input: "select /* join using */ 1 from t1 join t2 using (a)",
	}, {
		input: "select /* join using (a, b, c) */ 1 from t1 join t2 using (a, b, c)",
	}, {
		input: "select /* s.t */ 1 from s.t",
	}, {
		input: "select /* keyword schema & table name */ 1 from `By`.`bY`",
	}, {
		input: "select /* select in from */ 1 from (select 1 from t) as a",
	}, {
		input:  "select /* select in from with no as */ 1 from (select 1 from t) a",
		output: "select /* select in from with no as */ 1 from (select 1 from t) as a",
	}, {
		input: "select /* where */ 1 from t where a = b",
	}, {
		input: "select /* and */ 1 from t where a = b and a = c",
	}, {
		input:  "select /* && */ 1 from t where a = b && a = c",
		output: "select /* && */ 1 from t where a = b and a = c",
	}, {
		input: "select /* or */ 1 from t where a = b or a = c",
	}, {
		input:  "select /* || */ 1 from t where a = b || a = c",
		output: "select /* || */ 1 from t where a = b or a = c",
	}, {
		input: "select /* not */ 1 from t where not a = b",
	}, {
		input: "select /* ! */ 1 from t where a = !1",
	}, {
		input: "select /* bool is */ 1 from t where a = b is null",
	}, {
		input: "select /* bool is not */ 1 from t where a = b is not false",
	}, {
		input: "select /* true */ 1 from t where true",
	}, {
		input: "select /* false */ 1 from t where false",
	}, {
		input: "select /* false on left */ 1 from t where false = 0",
	}, {
		input: "select /* exists */ 1 from t where exists (select 1 from t)",
	}, {
		input:  "select /* (boolean) */ 1 from t where not (a = b)",
		output: "select /* (boolean) */ 1 from t where not a = b",
	}, {
		input: "select /* in value list */ 1 from t where a in (b, c)",
	}, {
		input: "select /* in select */ 1 from t where a in (select 1 from t)",
	}, {
		input: "select /* not in */ 1 from t where a not in (b, c)",
	}, {
		input: "select /* like */ 1 from t where a like b",
	}, {
		input: "select /* like escape */ 1 from t where a like b escape '!'",
	}, {
		input: "select /* not like */ 1 from t where a not like b",
	}, {
		input: "select /* not like escape */ 1 from t where a not like b escape '$'",
	}, {
		input: "select /* regexp */ 1 from t where a regexp b",
	}, {
		input: "select /* not regexp */ 1 from t where a not regexp b",
	}, {
		input:  "select /* rlike */ 1 from t where a rlike b",
		output: "select /* rlike */ 1 from t where a regexp b",
	}, {
		input:  "select /* not rlike */ 1 from t where a not rlike b",
		output: "select /* not rlike */ 1 from t where a not regexp b",
	}, {
		input: "select /* between */ 1 from t where a between b and c",
	}, {
		input: "select /* not between */ 1 from t where a not between b and c",
	}, {
		input: "select /* is null */ 1 from t where a is null",
	}, {
		input: "select /* is not null */ 1 from t where a is not null",
	}, {
		input: "select /* is true */ 1 from t where a is true",
	}, {
		input: "select /* is not true */ 1 from t where a is not true",
	}, {
		input: "select /* is false */ 1 from t where a is false",
	}, {
		input: "select /* is not false */ 1 from t where a is not false",
	}, {
		input: "select /* < */ 1 from t where a < b",
	}, {
		input: "select /* <= */ 1 from t where a <= b",
	}, {
		input: "select /* >= */ 1 from t where a >= b",
	}, {
		input: "select /* > */ 1 from t where a > b",
	}, {
		input: "select /* != */ 1 from t where a != b",
	}, {
		input:  "select /* <> */ 1 from t where a <> b",
		output: "select /* <> */ 1 from t where a != b",
	}, {
		input: "select /* <=> */ 1 from t where a <=> b",
	}, {
		input: "select /* != */ 1 from t where a != b",
	}, {
		input: "select /* single value expre list */ 1 from t where a in (b)",
	}, {
		input: "select /* select as a value expression */ 1 from t where a = (select a from t)",
	}, {
		input:  "select /* parenthesised value */ 1 from t where a = (b)",
		output: "select /* parenthesised value */ 1 from t where a = b",
	}, {
		input:  "select /* over-parenthesize */ ((1)) from t where ((a)) in (((1))) and ((a, b)) in ((((1, 1))), ((2, 2)))",
		output: "select /* over-parenthesize */ 1 from t where a in (1) and (a, b) in ((1, 1), (2, 2))",
	}, {
		input:  "select /* dot-parenthesize */ (a.b) from t where (b.c) = 2",
		output: "select /* dot-parenthesize */ a.b from t where b.c = 2",
	}, {
		input: "select /* & */ 1 from t where a = b & c",
	}, {
		input: "select /* & */ 1 from t where a = b & c",
	}, {
		input: "select /* | */ 1 from t where a = b | c",
	}, {
		input: "select /* ^ */ 1 from t where a = b ^ c",
	}, {
		input: "select /* + */ 1 from t where a = b + c",
	}, {
		input: "select /* - */ 1 from t where a = b - c",
	}, {
		input: "select /* * */ 1 from t where a = b * c",
	}, {
		input: "select /* / */ 1 from t where a = b / c",
	}, {
		input: "select /* % */ 1 from t where a = b % c",
	}, {
		input: "select /* div */ 1 from t where a = b div c",
	}, {
		input:  "select /* MOD */ 1 from t where a = b MOD c",
		output: "select /* MOD */ 1 from t where a = b % c",
	}, {
		input: "select /* << */ 1 from t where a = b << c",
	}, {
		input: "select /* >> */ 1 from t where a = b >> c",
	}, {
		input:  "select /* % no space */ 1 from t where a = b%c",
		output: "select /* % no space */ 1 from t where a = b % c",
	}, {
		input: "select /* u+ */ 1 from t where a = +b",
	}, {
		input: "select /* u- */ 1 from t where a = -b",
	}, {
		input: "select /* u~ */ 1 from t where a = ~b",
	}, {
		input: "select /* -> */ a.b -> 'ab' from t",
	}, {
		input: "select /* -> */ a.b ->> 'ab' from t",
	}, {
		input: "select /* empty function */ 1 from t where a = b()",
	}, {
		input: "select /* function with 1 param */ 1 from t where a = b(c)",
	}, {
		input: "select /* function with many params */ 1 from t where a = b(c, d)",
	}, {
		input: "select /* function with distinct */ count(distinct a) from t",
	}, {
		input:  "select count(distinctrow(1)) from (select (1) from dual union all select 1 from dual) a",
		output: "select count(distinct 1) from (select 1 from dual union all select 1 from dual) as a",
	}, {
		input: "select /* if as func */ 1 from t where a = if(b)",
	}, {
		input: "select /* current_timestamp */ current_timestamp() from t",
	}, {
		input: "select /* current_timestamp as func */ current_timestamp() from t",
	}, {
		input: "select /* current_timestamp with fsp */ current_timestamp(3) from t",
	}, {
		input: "select /* current_date */ current_date() from t",
	}, {
		input: "select /* current_date as func */ current_date() from t",
	}, {
		input: "select /* current_time */ current_time() from t",
	}, {
		input: "select /* current_time as func */ current_time() from t",
	}, {
		input: "select /* current_time with fsp */ current_time(1) from t",
	}, {
		input: "select /* utc_timestamp */ utc_timestamp() from t",
	}, {
		input: "select /* utc_timestamp as func */ utc_timestamp() from t",
	}, {
		input: "select /* utc_timestamp with fsp */ utc_timestamp(0) from t",
	}, {
		input: "select /* utc_time */ utc_time() from t",
	}, {
		input: "select /* utc_time as func */ utc_time() from t",
	}, {
		input: "select /* utc_time with fsp */ utc_time(4) from t",
	}, {
		input: "select /* utc_date */ utc_date() from t",
	}, {
		input: "select /* utc_date as func */ utc_date() from t",
	}, {
		input: "select /* localtime */ localtime() from t",
	}, {
		input: "select /* localtime as func */ localtime() from t",
	}, {
		input: "select /* localtime with fsp */ localtime(5) from t",
	}, {
		input: "select /* localtimestamp */ localtimestamp() from t",
	}, {
		input: "select /* localtimestamp as func */ localtimestamp() from t",
	}, {
		input: "select /* localtimestamp with fsp */ localtimestamp(7) from t",
	}, {
		input: "select /* mod as func */ a from tab where mod(b, 2) = 0",
	}, {
		input: "select /* database as func no param */ database() from t",
	}, {
		input: "select /* database as func 1 param */ database(1) from t",
	}, {
		input: "select /* a */ a from t",
	}, {
		input: "select /* a.b */ a.b from t",
	}, {
		input: "select /* a.b.c */ a.b.c from t",
	}, {
		input: "select /* keyword a.b */ `By`.`bY` from t",
	}, {
		input: "select /* string */ 'a' from t",
	}, {
		input:  "select /* double quoted string */ \"a\" from t",
		output: "select /* double quoted string */ 'a' from t",
	}, {
		input:  "select /* quote quote in string */ 'a''a' from t",
		output: "select /* quote quote in string */ 'a\\'a' from t",
	}, {
		input:  "select /* double quote quote in string */ \"a\"\"a\" from t",
		output: "select /* double quote quote in string */ 'a\\\"a' from t",
	}, {
		input:  "select /* quote in double quoted string */ \"a'a\" from t",
		output: "select /* quote in double quoted string */ 'a\\'a' from t",
	}, {
		input: "select /* backslash quote in string */ 'a\\'a' from t",
	}, {
		input: "select /* literal backslash in string */ 'a\\\\na' from t",
	}, {
		input: "select /* all escapes */ '\\0\\'\\\"\\b\\n\\r\\t\\Z\\\\' from t",
	}, {
		input:  "select /* non-escape */ '\\x' from t",
		output: "select /* non-escape */ 'x' from t",
	}, {
		input: "select /* unescaped backslash */ '\\n' from t",
	}, {
		input: "select /* value argument */ :a from t",
	}, {
		input: "select /* value argument with digit */ :a1 from t",
	}, {
		input: "select /* value argument with dot */ :a.b from t",
	}, {
		input:  "select /* positional argument */ ? from t",
		output: "select /* positional argument */ :v1 from t",
	}, {
		input:  "select /* multiple positional arguments */ ?, ? from t",
		output: "select /* multiple positional arguments */ :v1, :v2 from t",
	}, {
		input: "select /* list arg */ * from t where a in ::list",
	}, {
		input: "select /* list arg not in */ * from t where a not in ::list",
	}, {
		input: "select /* null */ null from t",
	}, {
		input: "select /* octal */ 010 from t",
	}, {
		input:  "select /* hex */ x'f0A1' from t",
		output: "select /* hex */ X'f0A1' from t",
	}, {
		input: "select /* hex caps */ X'F0a1' from t",
	}, {
		input:  "select /* bit literal */ b'0101' from t",
		output: "select /* bit literal */ B'0101' from t",
	}, {
		input: "select /* bit literal caps */ B'010011011010' from t",
	}, {
		input: "select /* 0x */ 0xf0 from t",
	}, {
		input: "select /* float */ 0.1 from t",
	}, {
		input: "select /* group by */ 1 from t group by a",
	}, {
		input: "select /* having */ 1 from t having a = b",
	}, {
		input:  "select /* simple order by */ 1 from t order by a",
		output: "select /* simple order by */ 1 from t order by a asc",
	}, {
		input: "select /* order by asc */ 1 from t order by a asc",
	}, {
		input: "select /* order by desc */ 1 from t order by a desc",
	}, {
		input: "select /* order by null */ 1 from t order by null",
	}, {
		input: "select /* limit a */ 1 from t limit a",
	}, {
		input: "select /* limit a,b */ 1 from t limit a, b",
	}, {
		input:  "select /* binary unary */ a- -b from t",
		output: "select /* binary unary */ a - -b from t",
	}, {
		input: "select /* - - */ - -b from t",
	}, {
		input: "select /* binary binary */ binary  binary b from t",
	}, {
		input: "select /* binary ~ */ binary  ~b from t",
	}, {
		input: "select /* ~ binary */ ~ binary b from t",
	}, {
		input: "select /* interval */ adddate('2008-01-02', interval 31 day) from t",
	}, {
		input: "select /* interval keyword */ adddate('2008-01-02', interval 1 year) from t",
	}, {
		input:  "select /* TIMESTAMPADD */ TIMESTAMPADD(MINUTE, 1, '2008-01-04') from t",
		output: "select /* TIMESTAMPADD */ timestampadd(MINUTE, 1, '2008-01-04') from t",
	}, {
		input:  "select /* TIMESTAMPDIFF */ TIMESTAMPDIFF(MINUTE, '2008-01-02', '2008-01-04') from t",
		output: "select /* TIMESTAMPDIFF */ timestampdiff(MINUTE, '2008-01-02', '2008-01-04') from t",
	}, {
		input: "select /* dual */ 1 from dual",
	}, {
		input:  "select /* Dual */ 1 from Dual",
		output: "select /* Dual */ 1 from dual",
	}, {
		input:  "select /* DUAL */ 1 from Dual",
		output: "select /* DUAL */ 1 from dual",
	}, {
		input: "select /* column as bool in where */ a from t where b",
	}, {
		input: "select /* OR of columns in where */ * from t where a or b",
	}, {
		input: "select /* OR of mixed columns in where */ * from t where a = 5 or b and c is not null",
	}, {
		input:  "select /* OR in select columns */ (a or b) from t where c = 5",
		output: "select /* OR in select columns */ a or b from t where c = 5",
	}, {
		input: "select /* XOR of columns in where */ * from t where a xor b",
	}, {
		input: "select /* XOR of mixed columns in where */ * from t where a = 5 xor b and c is not null",
	}, {
		input:  "select /* XOR in select columns */ (a xor b) from t where c = 5",
		output: "select /* XOR in select columns */ a xor b from t where c = 5",
	}, {
		input:  "select /* XOR in select columns */ * from t where (1 xor c1 > 0)",
		output: "select /* XOR in select columns */ * from t where 1 xor c1 > 0",
	}, {
		input: "select /* bool as select value */ a, true from t",
	}, {
		input: "select /* bool column in ON clause */ * from t join s on t.id = s.id and s.foo where t.bar",
	}, {
		input: "select /* bool in order by */ * from t order by a is null or b asc",
	}, {
		input: "select /* string in case statement */ if(max(case a when 'foo' then 1 else 0 end) = 1, 'foo', 'bar') as foobar from t",
	}, {
		input:  "/*!show databases*/",
		output: "show databases",
	}, {
		input:  "select /*!40101 * from*/ t",
		output: "select * from t",
	}, {
		input:  "select /*! * from*/ t",
		output: "select * from t",
	}, {
		input:  "select /*!* from*/ t",
		output: "select * from t",
	}, {
		input:  "select /*!401011 from*/ t",
		output: "select 1 from t",
	}, {
		input: "select /* dual */ 1 from dual",
	}, {
		input:  "select * from (select 'tables') tables",
		output: "select * from (select 'tables' from dual) as `tables`",
	}, {
		input: "insert /* simple */ into a values (1)",
	}, {
		input: "insert /* a.b */ into a.b values (1)",
	}, {
		input: "insert /* multi-value */ into a values (1, 2)",
	}, {
		input: "insert /* multi-value list */ into a values (1, 2), (3, 4)",
	}, {
		input: "insert /* no values */ into a values ()",
	}, {
		input:  "insert /* set */ into a set a = 1, b = 2",
		output: "insert /* set */ into a(a, b) values (1, 2)",
	}, {
		input:  "insert /* set default */ into a set a = default, b = 2",
		output: "insert /* set default */ into a(a, b) values (default, 2)",
	}, {
		input: "insert /* value expression list */ into a values (a + 1, 2 * 3)",
	}, {
		input: "insert /* default */ into a values (default, 2 * 3)",
	}, {
		input: "insert /* column list */ into a(a, b) values (1, 2)",
	}, {
		input: "insert into a(a, b) values (1, ifnull(null, default(b)))",
	}, {
		input: "insert /* qualified column list */ into a(a, b) values (1, 2)",
	}, {
		input:  "insert /* qualified columns */ into t (t.a, t.b) values (1, 2)",
		output: "insert /* qualified columns */ into t(a, b) values (1, 2)",
	}, {
		input: "insert /* select */ into a select b, c from d",
	}, {
		input:  "insert /* it accepts columns with keyword action */ into a(action, b) values (1, 2)",
		output: "insert /* it accepts columns with keyword action */ into a(`action`, b) values (1, 2)",
	}, {
		input: "insert /* no cols & paren select */ into a (select * from t)",
	}, {
		input: "insert /* cols & paren select */ into a(a, b, c) (select * from t)",
	}, {
		input: "insert /* cols & union with paren select */ into a(b, c) (select d, e from f) union (select g from h)",
	}, {
		input: "insert /* on duplicate */ into a values (1, 2) on duplicate key update b = func(a), c = d",
	}, {
		input: "insert /* bool in insert value */ into a values (1, true, false)",
	}, {
		input: "insert /* bool in on duplicate */ into a values (1, 2) on duplicate key update b = false, c = d",
	}, {
		input: "insert /* bool in on duplicate */ into a values (1, 2, 3) on duplicate key update b = values(b), c = d",
	}, {
		input: "insert /* bool in on duplicate */ into a values (1, 2, 3) on duplicate key update b = values(a.b), c = d",
	}, {
		input: "insert /* bool expression on duplicate */ into a values (1, 2) on duplicate key update b = func(a), c = a > d",
	}, {
		input: "insert into user(username, `status`) values ('Chuck', default(`status`))",
	}, {
		input:  "insert into user(format, tree, vitess) values ('Chuck', 42, 'Barry')",
		output: "insert into user(`format`, `tree`, `vitess`) values ('Chuck', 42, 'Barry')",
	}, {
		input:  "insert into customer () values ()",
		output: "insert into customer values ()",
	}, {
		input: "update /* simple */ a set b = 3",
	}, {
		input: "update /* a.b */ a.b set b = 3",
	}, {
		input: "update /* list */ a set b = 3, c = 4",
	}, {
		input: "update /* expression */ a set b = 3 + 4",
	}, {
		input: "update /* where */ a set b = 3 where a = b",
	}, {
		input: "update /* order */ a set b = 3 order by c desc",
	}, {
		input: "update /* limit */ a set b = 3 limit c",
	}, {
		input: "update /* bool in update */ a set b = true",
	}, {
		input: "update /* bool expr in update */ a set b = 5 > 2",
	}, {
		input: "update /* bool in update where */ a set b = 5 where c",
	}, {
		input: "update /* table qualifier */ a set a.b = 3",
	}, {
		input: "update /* table qualifier */ a set t.a.b = 3",
	}, {
		input:  "update /* table alias */ tt aa set aa.cc = 3",
		output: "update /* table alias */ tt as aa set aa.cc = 3",
	}, {
		input:  "update (select id from foo) subqalias set id = 4",
		output: "update (select id from foo) as subqalias set id = 4",
	}, {
		input:  "update foo f, bar b set f.id = b.id where b.name = 'test'",
		output: "update foo as f, bar as b set f.id = b.id where b.`name` = 'test'",
	}, {
		input:  "update foo f join bar b on f.name = b.name set f.id = b.id where b.name = 'test'",
		output: "update foo as f join bar as b on f.`name` = b.`name` set f.id = b.id where b.`name` = 'test'",
	}, {
		input: "update /* ignore */ ignore a set b = 3",
	}, {
		input: "delete /* simple */ from a",
	}, {
		input: "delete /* a.b */ from a.b",
	}, {
		input: "delete /* where */ from a where a = b",
	}, {
		input: "delete /* order */ from a order by b desc",
	}, {
		input: "delete /* limit */ from a limit b",
	}, {
		input:  "delete a from a join b on a.id = b.id where b.name = 'test'",
		output: "delete a from a join b on a.id = b.id where b.`name` = 'test'",
	}, {
		input:  "delete a, b from a, b where a.id = b.id and b.name = 'test'",
		output: "delete a, b from a, b where a.id = b.id and b.`name` = 'test'",
	}, {
		input: "delete /* simple */ ignore from a",
	}, {
		input: "delete ignore from a",
	}, {
		input: "delete /* limit */ ignore from a",
	}, {
		input:  "delete from a1, a2 using t1 as a1 inner join t2 as a2 where a1.id=a2.id",
		output: "delete a1, a2 from t1 as a1 join t2 as a2 where a1.id = a2.id",
	}, {
		input: "set /* simple */ a = 3",
	}, {
		input: "set #simple\n b = 4",
	}, {
		input: "set character_set_results = utf8",
	}, {
		input: "set @@session.autocommit = true",
	}, {
		input: "set @@session.`autocommit` = true",
	}, {
		input: "set @@session.'autocommit' = true",
	}, {
		input: "set @@session.\"autocommit\" = true",
	}, {
		input:  "set @@session.autocommit = ON",
		output: "set @@session.autocommit = 'on'",
	}, {
		input:  "set @@session.autocommit= OFF",
		output: "set @@session.autocommit = 'off'",
	}, {
		input:  "set autocommit = on",
		output: "set autocommit = 'on'",
	}, {
		input:  "set autocommit = off",
		output: "set autocommit = 'off'",
	}, {
		input:  "set names utf8 collate foo",
		output: "set names 'utf8'",
	}, {
		input:  "set names utf8 collate 'foo'",
		output: "set names 'utf8'",
	}, {
		input:  "set character set utf8",
		output: "set charset 'utf8'",
	}, {
		input:  "set character set 'utf8'",
		output: "set charset 'utf8'",
	}, {
		input:  "set character set \"utf8\"",
		output: "set charset 'utf8'",
	}, {
		input:  "set charset default",
		output: "set charset default",
	}, {
		input:  "set session wait_timeout = 3600",
		output: "set session wait_timeout = 3600",
	}, {
		input:  "set session wait_timeout = 3600, session autocommit = off",
		output: "set session wait_timeout = 3600, session autocommit = 'off'",
	}, {
		input:  "set session wait_timeout = 3600, @@global.autocommit = off",
		output: "set session wait_timeout = 3600, @@global.autocommit = 'off'",
	}, {
		input: "set /* list */ a = 3, b = 4",
	}, {
		input: "set /* mixed list */ a = 3, names 'utf8', charset 'ascii', b = 4",
	}, {
		input: "set session transaction isolation level repeatable read",
	}, {
		input: "set transaction isolation level repeatable read",
	}, {
		input: "set global transaction isolation level repeatable read",
	}, {
		input: "set transaction isolation level repeatable read",
	}, {
		input: "set transaction isolation level read committed",
	}, {
		input: "set transaction isolation level read uncommitted",
	}, {
		input: "set transaction isolation level serializable",
	}, {
		input: "set transaction read write",
	}, {
		input: "set transaction read only",
	}, {
		input: "set tx_read_only = 1",
	}, {
		input: "set tx_read_only = 0",
	}, {
		input: "set transaction_read_only = 1",
	}, {
		input: "set transaction_read_only = 0",
	}, {
		input: "set tx_isolation = 'repeatable read'",
	}, {
		input: "set tx_isolation = 'read committed'",
	}, {
		input: "set tx_isolation = 'read uncommitted'",
	}, {
		input: "set tx_isolation = 'serializable'",
	}, {
		input: "set sql_safe_updates = 0",
	}, {
		input: "set sql_safe_updates = 1",
	}, {
		input: "set @variable = 42",
	}, {
		input: "set @period.variable = 42",
	}, {
		input:  "alter table a add foo",
		output: "alter table a",
	}, {
		input: "alter table a add spatial key foo (column1)",
	}, {
		input: "alter table a add fulltext key foo (column1)",
	}, {
		input: "alter table a add unique key foo (column1)",
	}, {
		input:  "alter table `By` add foo",
		output: "alter table `By`",
	}, {
		input:  "alter table a alter foo",
		output: "alter table a",
	}, {
		input:  "alter table a change foo",
		output: "alter table a",
	}, {
		input:  "alter table a modify foo",
		output: "alter table a",
	}, {
		input:  "alter table a drop foo",
		output: "alter table a",
	}, {
		input:  "alter table a disable foo",
		output: "alter table a",
	}, {
		input:  "alter table a enable foo",
		output: "alter table a",
	}, {
		input:  "alter table a order foo",
		output: "alter table a",
	}, {
		input:  "alter table a default foo",
		output: "alter table a",
	}, {
		input:  "alter table a discard foo",
		output: "alter table a",
	}, {
		input:  "alter table a import foo",
		output: "alter table a",
	}, {
		input:  "alter table a rename b",
		output: "rename table a to b",
	}, {
		input:  "alter table `By` rename `bY`",
		output: "rename table `By` to `bY`",
	}, {
		input:  "alter table a rename to b",
		output: "rename table a to b",
	}, {
		input:  "alter table a rename as b",
		output: "rename table a to b",
	}, {
		input:  "alter table a rename index foo to bar",
		output: "alter table a",
	}, {
		input:  "alter table a rename key foo to bar",
		output: "alter table a",
	}, {
		input:  "alter table e auto_increment = 20",
		output: "alter table e",
	}, {
		input:  "alter table e character set = 'ascii'",
		output: "alter table e",
	}, {
		input:  "alter table e default character set = 'ascii'",
		output: "alter table e",
	}, {
		input:  "alter table e comment = 'hello'",
		output: "alter table e",
	}, {
		input:  "alter table a reorganize partition b into (partition c values less than (?), partition d values less than (maxvalue))",
		output: "alter table a reorganize partition b into (partition c values less than (:v1), partition d values less than (maxvalue))",
	}, {
		input:  "alter table a partition by range (id) (partition p0 values less than (10), partition p1 values less than (maxvalue))",
		output: "alter table a",
	}, {
		input: "alter table a add column (id int, id2 char(23))",
	}, {
		input: "alter table a add index idx (id)",
	}, {
		input: "alter table a add fulltext index idx (id)",
	}, {
		input: "alter table a add spatial index idx (id)",
	}, {
		input: "alter table a add fulltext index idx (id)",
	}, {
		input: "alter table a add foreign key (id) references f (id)",
	}, {
		input: "alter table a add primary key (id)",
	}, {
		input: "alter table a add constraint b primary key (id)",
	}, {
		input: "alter table a add constraint b primary key (id)",
	}, {
		input: "alter table a add constraint b unique key (id)",
	}, {
		input:  "alter table a add constraint b unique c (id)",
		output: "alter table a add constraint b unique key c (id)",
	}, {
		input:  "alter table a add constraint check (id)",
		output: "alter table a add check (id) enforced",
	}, {
		input:  "alter table a add id int",
		output: "alter table a add column id int",
	}, {
		input: "alter table a add column id int first id2",
	}, {
		input: "alter table a add column id int after id2",
	}, {
		input:  "alter table a drop column id int",
		output: "alter table a",
	}, {
		input:  "alter table a drop partition p2712",
		output: "alter table a",
	}, {
		input:  "alter table a drop index idx (id)",
		output: "alter table a",
	}, {
		input:  "alter table a drop fulltext index idx (id)",
		output: "alter table a",
	}, {
		input:  "alter table a drop spatial index idx (id)",
		output: "alter table a",
	}, {
		input:  "alter table a drop fulltext index idx (id)",
		output: "alter table a",
	}, {
		input: "alter table a add check (ch_1) not enforced",
	}, {
		input:  "alter table a drop check ch_1",
		output: "alter table a",
	}, {
		input:  "alter table a drop foreign key",
		output: "alter table a",
	}, {
		input:  "alter table a drop primary key",
		output: "alter table a",
	}, {
		input:  "alter table a drop constraint",
		output: "alter table a",
	}, {
		input:  "alter table a drop id",
		output: "alter table a",
	}, {
		input: "alter database character set geostd8",
	}, {
		input: "alter database d character set geostd8",
	}, {
		input: "alter database d default collate 'utf8_bin'",
	}, {
		input: "alter database default collate 'utf8_bin'",
	}, {
		input: "alter database d upgrade data directory name",
	}, {
		input:  "alter database d collate = 'utf8_bin'",
		output: "alter database d collate 'utf8_bin'",
	}, {
		input:  "alter schema d default character set = geostd8",
		output: "alter database d default character set geostd8",
	}, {
		input:  "alter schema d character set = geostd8",
		output: "alter database d character set geostd8",
	}, {
		input:  "alter schema d default collate = 'utf8_bin'",
		output: "alter database d default collate 'utf8_bin'",
	}, {
		input:  "alter schema d collate = 'utf8_bin' character set = geostd8 character set = geostd8",
		output: "alter database d collate 'utf8_bin' character set geostd8 character set geostd8",
	}, {
		input: "create table a",
	}, {
		input:  "create table a (\n\t`a` int\n)",
		output: "create table a (\n\ta int\n)",
	}, {
		input: "create table `by` (\n\t`by` char\n)",
	}, {
		input: "create table test (\n\t__year year(4)\n)",
	}, {
		input:  "create table if not exists a (\n\t`a` int\n)",
		output: "create table a (\n\ta int\n)",
	}, {
		input:  "create table a ignore me this is garbage",
		output: "create table a",
	}, {
		input:  "create table a (a int, b char, c garbage)",
		output: "create table a",
	}, {
		input:  "create table a (b1 bool not null primary key, b2 boolean not null)",
		output: "create table a (\n\tb1 bool not null primary key,\n\tb2 boolean not null\n)",
	}, {
		input: "alter vschema create vindex hash_vdx using hash",
	}, {
		input: "alter vschema create vindex keyspace.hash_vdx using hash",
	}, {
		input: "alter vschema create vindex lookup_vdx using lookup with owner=user, table=name_user_idx, from=name, to=user_id",
	}, {
		input: "alter vschema create vindex xyz_vdx using xyz with param1=hello, param2='world', param3=123",
	}, {
		input: "alter vschema drop vindex hash_vdx",
	}, {
		input: "alter vschema drop vindex ks.hash_vdx",
	}, {
		input: "alter vschema add table a",
	}, {
		input: "alter vschema add table ks.a",
	}, {
		input: "alter vschema add sequence a_seq",
	}, {
		input: "alter vschema add sequence ks.a_seq",
	}, {
		input: "alter vschema on a add auto_increment id using a_seq",
	}, {
		input: "alter vschema on ks.a add auto_increment id using a_seq",
	}, {
		input: "alter vschema drop table a",
	}, {
		input: "alter vschema drop table ks.a",
	}, {
		input: "alter vschema on a add vindex hash (id)",
	}, {
		input: "alter vschema on ks.a add vindex hash (id)",
	}, {
		input:  "alter vschema on a add vindex `hash` (`id`)",
		output: "alter vschema on a add vindex hash (id)",
	}, {
		input:  "alter vschema on `ks`.a add vindex `hash` (`id`)",
		output: "alter vschema on ks.a add vindex hash (id)",
	}, {
		input:  "alter vschema on a add vindex hash (id) using `hash`",
		output: "alter vschema on a add vindex hash (id) using hash",
	}, {
		input: "alter vschema on a add vindex `add` (`add`)",
	}, {
		input: "alter vschema on a add vindex hash (id) using hash",
	}, {
		input:  "alter vschema on a add vindex hash (id) using `hash`",
		output: "alter vschema on a add vindex hash (id) using hash",
	}, {
		input:  "alter vschema on user add vindex name_lookup_vdx (name) using lookup_hash with owner=user, table=name_user_idx, from=name, to=user_id",
		output: "alter vschema on user add vindex name_lookup_vdx (`name`) using lookup_hash with owner=user, table=name_user_idx, from=name, to=user_id",
	}, {
		input:  "alter vschema on user2 add vindex name_lastname_lookup_vdx (name,lastname) using lookup with owner=`user`, table=`name_lastname_keyspace_id_map`, from=`name,lastname`, to=`keyspace_id`",
		output: "alter vschema on user2 add vindex name_lastname_lookup_vdx (`name`, lastname) using lookup with owner=user, table=name_lastname_keyspace_id_map, from=name,lastname, to=keyspace_id",
	}, {
		input: "alter vschema on a drop vindex hash",
	}, {
		input: "alter vschema on ks.a drop vindex hash",
	}, {
		input:  "alter vschema on a drop vindex `hash`",
		output: "alter vschema on a drop vindex hash",
	}, {
		input:  "alter vschema on a drop vindex hash",
		output: "alter vschema on a drop vindex hash",
	}, {
		input:  "alter vschema on a drop vindex `add`",
		output: "alter vschema on a drop vindex `add`",
	}, {
		input:  "create index a on b (col1)",
		output: "alter table b add index a (col1)",
	}, {
		input:  "create unique index a on b (col1)",
		output: "alter table b add unique index a (col1)",
	}, {
		input:  "create unique index a using foo on b (col1 desc)",
		output: "alter table b add unique index a (col1 desc) using foo",
	}, {
		input:  "create fulltext index a on b (col1) with parser a",
		output: "alter table b add fulltext index a (col1) with parser a",
	}, {
		input:  "create spatial index a on b (col1)",
		output: "alter table b add spatial index a (col1)",
	}, {
		input:  "create fulltext index a on b (col1) key_block_size=12 with parser a comment 'string' algorithm inplace lock none",
		output: "alter table b add fulltext index a (col1) key_block_size 12 with parser a comment 'string' algorithm inplace lock none",
	}, {
		input:      "create index a on b ((col1 + col2), (col1*col2))",
		output:     "alter table b add index a ()",
		partialDDL: true,
	}, {
		input: "create algorithm = merge sql security definer view a as select * from e",
	}, {
		input: "create view ks.a as select * from e",
	}, {
		input:  "create algorithm = merge sql security definer view a (b,c,d) as select * from e",
		output: "create algorithm = merge sql security definer view a(b, c, d) as select * from e",
	}, {
		input:  "create algorithm = merge sql security definer view a (b,c,d) as select * from e with cascaded check option",
		output: "create algorithm = merge sql security definer view a(b, c, d) as select * from e with cascaded check option",
	}, {
		input:  "create algorithm = temptable definer = a@b.c.d view a(b,c,d) as select * from e with local check option",
		output: "create algorithm = temptable definer = a@b.c.d view a(b, c, d) as select * from e with local check option",
	}, {
		input:  "create or replace algorithm = temptable definer = a@b.c.d sql security definer view a(b,c,d) as select * from e with local check option",
		output: "create or replace algorithm = temptable definer = a@b.c.d sql security definer view a(b, c, d) as select * from e with local check option",
	}, {
		input:  "create definer = 'sa'@b.c.d view a(b,c,d) as select * from e",
		output: "create definer = 'sa'@b.c.d view a(b, c, d) as select * from e",
	}, {
		input:  "alter view a",
		output: "alter table a",
	}, {
		input:  "rename table a to b",
		output: "rename table a to b",
	}, {
		input:  "rename table a to b, b to c",
		output: "rename table a to b, b to c",
	}, {
		input:  "drop view a",
		output: "drop table a",
	}, {
		input:  "drop table a",
		output: "drop table a",
	}, {
		input:  "drop table a, b",
		output: "drop table a, b",
	}, {
		input:  "drop table if exists a",
		output: "drop table if exists a",
	}, {
		input:  "drop view if exists a",
		output: "drop table if exists a",
	}, {
		input:  "drop index b on a",
		output: "alter table a",
	}, {
		input:  "analyze table a",
		output: "otherread",
	}, {
		input:  "flush tables",
		output: "flush",
	}, {
		input:  "flush tables with read lock",
		output: "flush",
	}, {
		input:  "show binary logs",
		output: "show binary logs",
	}, {
		input:  "show binlog events",
		output: "show binlog",
	}, {
		input:  "show character set",
		output: "show charset",
	}, {
		input:  "show character set like '%foo'",
		output: "show charset like '%foo'",
	}, {
		input:  "show charset",
		output: "show charset",
	}, {
		input:  "show charset like '%foo'",
		output: "show charset like '%foo'",
	}, {
		input:  "show charset where 'charset' = 'utf8'",
		output: "show charset where 'charset' = 'utf8'",
	}, {
		input:  "show charset where 'charset' = '%foo'",
		output: "show charset where 'charset' = '%foo'",
	}, {
		input:  "show collation",
		output: "show collation",
	}, {
		input:  "show collation where `Charset` = 'utf8' and `Collation` = 'utf8_bin'",
		output: "show collation where `Charset` = 'utf8' and `Collation` = 'utf8_bin'",
	}, {
		input:  "show create database d",
		output: "show create database",
	}, {
		input:  "show create event e",
		output: "show create event",
	}, {
		input: "show create function f",
	}, {
		input:  "show create procedure p",
		output: "show create procedure",
	}, {
		input:  "show create table t",
		output: "show create table t",
	}, {
		input:  "show create trigger t",
		output: "show create trigger",
	}, {
		input:  "show create user u",
		output: "show create user",
	}, {
		input:  "show create view v",
		output: "show create view",
	}, {
		input:  "show databases",
		output: "show databases",
	}, {
		input:  "show databases like '%'",
		output: "show databases like '%'",
	}, {
		input:  "show schemas",
		output: "show databases",
	}, {
		input:  "show schemas like '%'",
		output: "show databases like '%'",
	}, {
		input:  "show engine INNODB",
		output: "show engine",
	}, {
		input: "show engines",
	}, {
		input:  "show storage engines",
		output: "show storage",
	}, {
		input: "show errors",
	}, {
		input: "show events",
	}, {
		input: "show function code func",
	}, {
		input: "show function status",
	}, {
		input:  "show grants for 'root@localhost'",
		output: "show grants",
	}, {
		input: "show index from t",
	}, {
		input: "show indexes from t",
	}, {
		input: "show keys from t",
	}, {
		input:  "show master status",
		output: "show master",
	}, {
		input:  "show open tables",
		output: "show open",
	}, {
		input:  "show plugins",
		output: "show plugins",
	}, {
		input:  "show privileges",
		output: "show privileges",
	}, {
		input: "show procedure code p",
	}, {
		input: "show procedure status",
	}, {
		input:  "show processlist",
		output: "show processlist",
	}, {
		input:  "show full processlist",
		output: "show processlist",
	}, {
		input:  "show profile cpu for query 1",
		output: "show profile",
	}, {
		input:  "show profiles",
		output: "show profiles",
	}, {
		input:  "show relaylog events",
		output: "show relaylog",
	}, {
		input:  "show slave hosts",
		output: "show slave",
	}, {
		input:  "show slave status",
		output: "show slave",
	}, {
		input:  "show status",
		output: "show status",
	}, {
		input:  "show global status",
		output: "show global status",
	}, {
		input:  "show session status",
		output: "show status",
	}, {
		input: "show table status",
	}, {
		input: "show table status from dbname",
	}, {
		input:  "show table status in dbname",
		output: "show table status from dbname",
	}, {
		input:  "show table status in dbname LIKE '%' ",
		output: "show table status from dbname like '%'",
	}, {
		input:  "show table status from dbname Where col=42 ",
		output: "show table status from dbname where col = 42",
	}, {
		input: "show tables",
	}, {
		input: "show tables like '%keyspace%'",
	}, {
		input: "show tables where 1 = 0",
	}, {
		input: "show tables from a",
	}, {
		input: "show tables from a where 1 = 0",
	}, {
		input: "show tables from a like '%keyspace%'",
	}, {
		input: "show full tables",
	}, {
		input: "show full tables from a",
	}, {
		input:  "show full tables in a",
		output: "show full tables from a",
	}, {
		input: "show full tables from a like '%keyspace%'",
	}, {
		input: "show full tables from a where 1 = 0",
	}, {
		input: "show full tables like '%keyspace%'",
	}, {
		input: "show full tables where 1 = 0",
	}, {
		input:  "show full columns in a in b like '%'",
		output: "show full columns from a from b like '%'",
	}, {
		input: "show full columns from messages from test_keyspace like '%'",
	}, {
		input:  "show full fields from a like '%'",
		output: "show full columns from a like '%'",
	}, {
		input:  "show fields from a where 1 = 1",
		output: "show columns from a where 1 = 1",
	}, {
		input:  "show triggers",
		output: "show triggers",
	}, {
		input:  "show variables",
		output: "show variables",
	}, {
		input:  "show global variables",
		output: "show global variables",
	}, {
		input:  "show session variables",
		output: "show variables",
	}, {
		input:  "show vitess_keyspaces",
		output: "show databases",
	}, {
		input:  "show vitess_keyspaces like '%'",
		output: "show databases like '%'",
	}, {
		input: "show vitess_shards",
	}, {
		input: "show vitess_shards like '%'",
	}, {
		input: "show vitess_tablets",
	}, {
		input: "show vitess_tablets like '%'",
	}, {
		input: "show vitess_tablets where hostname = 'some-tablet'",
	}, {
		input: "show vschema tables",
	}, {
		input: "show vschema vindexes",
	}, {
		input: "show vschema vindexes on t",
	}, {
		input:  "show warnings",
		output: "show warnings",
	}, {
		input:  "select warnings from t",
		output: "select `warnings` from t",
	}, {
		input:  "show foobar",
		output: "show foobar",
	}, {
		input:  "show foobar like select * from table where syntax is 'ignored'",
		output: "show foobar",
	}, {
		input:  "use db",
		output: "use db",
	}, {
		input:  "use duplicate",
		output: "use `duplicate`",
	}, {
		input:  "use `ks:-80@master`",
		output: "use `ks:-80@master`",
	}, {
		input:  "use @replica",
		output: "use `@replica`",
	}, {
		input:  "use ks@replica",
		output: "use `ks@replica`",
	}, {
		input:  "describe select * from t",
		output: "explain select * from t",
	}, {
		input:  "desc select * from t",
		output: "explain select * from t",
	}, {
		input:  "desc foobar",
		output: "otherread",
	}, {
		input:  "explain t1",
		output: "otherread",
	}, {
		input:  "explain t1 col",
		output: "otherread",
	}, {
		input: "explain select * from t",
	}, {
		input: "explain format = traditional select * from t",
	}, {
		input: "explain analyze select * from t",
	}, {
		input: "explain format = tree select * from t",
	}, {
		input: "explain format = json select * from t",
	}, {
		input: "explain format = vitess select * from t",
	}, {
		input:  "describe format = vitess select * from t",
		output: "explain format = vitess select * from t",
	}, {
		input: "explain delete from t",
	}, {
		input: "explain insert into t(col1, col2) values (1, 2)",
	}, {
		input: "explain update t set col = 2",
	}, {
		input:  "truncate table foo",
		output: "truncate table foo",
	}, {
		input:  "truncate foo",
		output: "truncate table foo",
	}, {
		input:  "repair foo",
		output: "otheradmin",
	}, {
		input:  "optimize foo",
		output: "otheradmin",
	}, {
		input:  "lock tables foo read",
		output: "lock tables foo read",
	}, {
		input:  "lock tables foo write",
		output: "lock tables foo write",
	}, {
		input:  "lock tables foo read local",
		output: "lock tables foo read local",
	}, {
		input:  "lock tables foo low_priority write",
		output: "lock tables foo low_priority write",
	}, {
		input:  "unlock tables",
		output: "unlock tables",
	}, {
		input: "select /* EQ true */ 1 from t where a = true",
	}, {
		input: "select /* EQ false */ 1 from t where a = false",
	}, {
		input: "select /* NE true */ 1 from t where a != true",
	}, {
		input: "select /* NE false */ 1 from t where a != false",
	}, {
		input: "select /* LT true */ 1 from t where a < true",
	}, {
		input: "select /* LT false */ 1 from t where a < false",
	}, {
		input: "select /* GT true */ 1 from t where a > true",
	}, {
		input: "select /* GT false */ 1 from t where a > false",
	}, {
		input: "select /* LE true */ 1 from t where a <= true",
	}, {
		input: "select /* LE false */ 1 from t where a <= false",
	}, {
		input: "select /* GE true */ 1 from t where a >= true",
	}, {
		input: "select /* GE false */ 1 from t where a >= false",
	}, {
		input:  "select * from t order by a collate utf8_general_ci",
		output: "select * from t order by a collate utf8_general_ci asc",
	}, {
		input: "select k collate latin1_german2_ci as k1 from t1 order by k1 asc",
	}, {
		input: "select * from t group by a collate utf8_general_ci",
	}, {
		input: "select MAX(k collate latin1_german2_ci) from t1",
	}, {
		input: "select distinct k collate latin1_german2_ci from t1",
	}, {
		input: "select * from t1 where 'Müller' collate latin1_german2_ci = k",
	}, {
		input: "select * from t1 where k like 'Müller' collate latin1_german2_ci",
	}, {
		input: "select k from t1 group by k having k = 'Müller' collate latin1_german2_ci",
	}, {
		input: "select k from t1 join t2 order by a collate latin1_german2_ci asc, b collate latin1_german2_ci asc",
	}, {
		input:  "select k collate 'latin1_german2_ci' as k1 from t1 order by k1 asc",
		output: "select k collate latin1_german2_ci as k1 from t1 order by k1 asc",
	}, {
		input:  "select /* drop trailing semicolon */ 1 from dual;",
		output: "select /* drop trailing semicolon */ 1 from dual",
	}, {
		input: "select /* cache directive */ sql_no_cache 'foo' from t",
	}, {
		input: "select distinct sql_no_cache 'foo' from t",
	}, {
		input:  "select sql_no_cache distinct 'foo' from t",
		output: "select distinct sql_no_cache 'foo' from t",
	}, {
		input:  "select sql_no_cache straight_join distinct 'foo' from t",
		output: "select distinct sql_no_cache straight_join 'foo' from t",
	}, {
		input:  "select straight_join distinct sql_no_cache 'foo' from t",
		output: "select distinct sql_no_cache straight_join 'foo' from t",
	}, {
		input:  "select sql_calc_found_rows 'foo' from t",
		output: "select sql_calc_found_rows 'foo' from t",
	}, {
		input: "select binary 'a' = 'A' from t",
	}, {
		input: "select 1 from t where foo = _binary 'bar'",
	}, {
		input: "select 1 from t where foo = _utf8 'bar' and bar = _latin1 'sjösjuk'",
	}, {
		input:  "select 1 from t where foo = _binary'bar'",
		output: "select 1 from t where foo = _binary 'bar'",
	}, {
		input: "select 1 from t where foo = _utf8mb4 'bar'",
	}, {
		input:  "select 1 from t where foo = _utf8mb4'bar'",
		output: "select 1 from t where foo = _utf8mb4 'bar'",
	}, {
		input: "select match(a) against ('foo') from t",
	}, {
		input: "select match(a1, a2) against ('foo' in natural language mode with query expansion) from t",
	}, {
		input:  "select database()",
		output: "select database() from dual",
	}, {
		input:  "select schema()",
		output: "select schema() from dual",
	}, {
		input: "select title from video as v where match(v.title, v.tag) against ('DEMO' in boolean mode)",
	}, {
		input:  "select name, group_concat(score) from t group by name",
		output: "select `name`, group_concat(score) from t group by `name`",
	}, {
		input:  "select name, group_concat(distinct id, score order by id desc separator ':') from t group by name",
		output: "select `name`, group_concat(distinct id, score order by id desc separator ':') from t group by `name`",
	}, {
		input:  "select name, group_concat(distinct id, score order by id desc separator ':' limit 1) from t group by name",
		output: "select `name`, group_concat(distinct id, score order by id desc separator ':' limit 1) from t group by `name`",
	}, {
		input:  "select name, group_concat(distinct id, score order by id desc separator ':' limit 10, 2) from t group by name",
		output: "select `name`, group_concat(distinct id, score order by id desc separator ':' limit 10, 2) from t group by `name`",
	}, {
		input: "select * from t partition (p0)",
	}, {
		input: "select * from t partition (p0, p1)",
	}, {
		input: "select e.id, s.city from employees as e join stores partition (p1) as s on e.store_id = s.id",
	}, {
		input: "select truncate(120.3333, 2) from dual",
	}, {
		input: "update t partition (p0) set a = 1",
	}, {
		input: "insert into t partition (p0) values (1, 'asdf')",
	}, {
		input: "insert into t1 select * from t2 partition (p0)",
	}, {
		input: "replace into t partition (p0) values (1, 'asdf')",
	}, {
		input: "delete from t partition (p0) where a = 1",
	}, {
		input: "stream * from t",
	}, {
		input: "vstream * from t",
	}, {
		input: "stream /* comment */ * from t",
	}, {
		input: "begin",
	}, {
		input:  "begin;",
		output: "begin",
	}, {
		input:  "start transaction",
		output: "begin",
	}, {
		input: "commit",
	}, {
		input: "rollback",
	}, {
		input: "create database test_db",
	}, {
		input:  "create schema test_db",
		output: "create database test_db",
	}, {
		input: "create database if not exists test_db",
	}, {
		input:  "create schema if not exists test_db",
		output: "create database if not exists test_db",
	}, {
		input: "create database test_db default collate 'utf8mb4_general_ci' collate utf8mb4_general_ci",
	}, {
		input: "create database test_db character set geostd8",
	}, {
		input:      "create database test_db character set * unparsable",
		output:     "create database test_db",
		partialDDL: true,
	}, {
		input: "drop database test_db",
	}, {
		input:  "drop schema test_db",
		output: "drop database test_db",
	}, {
		input: "drop database if exists test_db",
	}, {
		input:  "delete a.*, b.* from tbl_a a, tbl_b b where a.id = b.id and b.name = 'test'",
		output: "delete a, b from tbl_a as a, tbl_b as b where a.id = b.id and b.`name` = 'test'",
	}, {
		input:  "select distinctrow a.* from (select (1) from dual union all select 1 from dual) a",
		output: "select distinct a.* from (select 1 from dual union all select 1 from dual) as a",
	}, {
		input: "select `weird function name`() from t",
	}, {
		input: "select status() from t", // should not escape function names that are keywords
	}, {
		input: "select * from `weird table name`",
	}, {
		input:  "SHOW FULL TABLES FROM `jiradb` LIKE 'AO_E8B6CC_ISSUE_MAPPING'",
		output: "show full tables from jiradb like 'AO_E8B6CC_ISSUE_MAPPING'",
	}, {
		input:  "SHOW FULL COLUMNS FROM AO_E8B6CC_ISSUE_MAPPING FROM jiradb LIKE '%'",
		output: "show full columns from AO_E8B6CC_ISSUE_MAPPING from jiradb like '%'",
	}, {
		input:  "SHOW KEYS FROM `AO_E8B6CC_ISSUE_MAPPING` FROM `jiradb`",
		output: "show keys from AO_E8B6CC_ISSUE_MAPPING from jiradb",
	}, {
		input:  "SHOW CREATE TABLE `jiradb`.`AO_E8B6CC_ISSUE_MAPPING`",
		output: "show create table jiradb.AO_E8B6CC_ISSUE_MAPPING",
	}, {
		input:  "SHOW INDEX FROM `AO_E8B6CC_ISSUE_MAPPING` FROM `jiradb`",
		output: "show index from AO_E8B6CC_ISSUE_MAPPING from jiradb",
	}, {
		input:  "SHOW FULL TABLES FROM `jiradb` LIKE '%'",
		output: "show full tables from jiradb like '%'",
	}, {
		input:  "SHOW EXTENDED INDEX FROM `AO_E8B6CC_PROJECT_MAPPING` FROM `jiradb`",
		output: "show extended index from AO_E8B6CC_PROJECT_MAPPING from jiradb",
	}, {
		input:  "SHOW EXTENDED KEYS FROM `AO_E8B6CC_ISSUE_MAPPING` FROM `jiradb`",
		output: "show extended keys from AO_E8B6CC_ISSUE_MAPPING from jiradb",
	}, {
		input:  "SHOW CREATE TABLE `jiradb`.`AO_E8B6CC_ISSUE_MAPPING`",
		output: "show create table jiradb.AO_E8B6CC_ISSUE_MAPPING",
	}, {
		input: "create table t1 ( check (c1 <> c2), c1 int check (c1 > 10), c2 int constraint c2_positive check (c2 > 0), c3 int check (c3 < 100), constraint c1_nonzero check (c1 <> 0), check (c1 > c3))",
		output: "create table t1 (\n" +
			"\tc1 int,\n" +
			"\tc2 int,\n" +
			"\tc3 int,\n" +
			"\tcheck (c1 != c2) enforced,\n" +
			"\tcheck (c1 > 10) enforced,\n" +
			"\tconstraint c2_positive check (c2 > 0) enforced,\n" +
			"\tcheck (c3 < 100) enforced,\n" +
			"\tconstraint c1_nonzero check (c1 != 0) enforced,\n" +
			"\tcheck (c1 > c3) enforced\n)",
	}, {
		input:  "SHOW INDEXES FROM `AO_E8B6CC_ISSUE_MAPPING` FROM `jiradb`",
		output: "show indexes from AO_E8B6CC_ISSUE_MAPPING from jiradb",
	}, {
		input:  "SHOW FULL TABLES FROM `jiradb` LIKE '%'",
		output: "show full tables from jiradb like '%'",
	}, {
		input:  "SHOW EXTENDED INDEXES FROM `AO_E8B6CC_PROJECT_MAPPING` FROM `jiradb`",
		output: "show extended indexes from AO_E8B6CC_PROJECT_MAPPING from jiradb",
	}, {
		input:  "SHOW EXTENDED INDEXES IN `AO_E8B6CC_PROJECT_MAPPING` IN `jiradb`",
		output: "show extended indexes from AO_E8B6CC_PROJECT_MAPPING from jiradb",
	}, {
		input:  "do 1",
		output: "otheradmin",
	}, {
		input:  "do funcCall(), 2 = 1, 3 + 1",
		output: "otheradmin",
	}, {
		input: "savepoint a",
	}, {
		input: "savepoint `@@@;a`",
	}, {
		input: "rollback to a",
	}, {
		input: "rollback to `@@@;a`",
	}, {
		input:  "rollback work to a",
		output: "rollback to a",
	}, {
		input:  "rollback to savepoint a",
		output: "rollback to a",
	}, {
		input:  "rollback work to savepoint a",
		output: "rollback to a",
	}, {
		input: "release savepoint a",
	}, {
		input: "release savepoint `@@@;a`",
	}}
)

func TestValid(t *testing.T) {
	for _, tcase := range validSQL {
		t.Run(tcase.input, func(t *testing.T) {
			if tcase.output == "" {
				tcase.output = tcase.input
			}
			tree, err := Parse(tcase.input)
			require.NoError(t, err, tcase.input)
			out := String(tree)
			if tcase.output != out {
				t.Errorf("Parsing failed. \nExpected/Got:\n%s\n%s", tcase.output, out)
			}

			// CREATE INDEX currently only has 5.7 specifications.
			// For mysql 8.0 syntax, the query is not entirely parsed.
			// Add more structs as we go on adding full parsing support for DDL constructs for 5.7 syntax.
			switch x := tree.(type) {
			case *CreateIndex:
				assert.Equal(t, !tcase.partialDDL, x.IsFullyParsed())
			case *CreateDatabase:
				assert.Equal(t, !tcase.partialDDL, x.IsFullyParsed())
			case *AlterDatabase:
				assert.Equal(t, !tcase.partialDDL, x.IsFullyParsed())
			}
			// This test just exercises the tree walking functionality.
			// There's no way automated way to verify that a node calls
			// all its children. But we can examine code coverage and
			// ensure that all walkSubtree functions were called.
			Walk(func(node SQLNode) (bool, error) {
				return true, nil
			}, tree)
		})
	}
}

// Ensure there is no corruption from using a pooled yyParserImpl in Parse.
func TestParallelValid(t *testing.T) {
	parallelism := 100
	numIters := 1000

	wg := sync.WaitGroup{}
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIters; j++ {
				tcase := validSQL[rand.Intn(len(validSQL))]
				if tcase.output == "" {
					tcase.output = tcase.input
				}
				tree, err := Parse(tcase.input)
				if err != nil {
					t.Errorf("Parse(%q) err: %v, want nil", tcase.input, err)
					continue
				}
				out := String(tree)
				if out != tcase.output {
					t.Errorf("Parse(%q) = %q, want: %q", tcase.input, out, tcase.output)
				}
			}
		}()
	}
	wg.Wait()
}

func TestInvalid(t *testing.T) {
	invalidSQL := []struct {
		input string
		err   string
	}{{
		input: "select a, b from (select * from tbl) sort by a",
		err:   "syntax error",
	}, {
		input: "/*!*/",
		err:   "empty statement",
	}}

	for _, tcase := range invalidSQL {
		_, err := Parse(tcase.input)
		if err == nil {
			t.Errorf("Parse invalid query(%q), got: nil, want: %s...", tcase.input, tcase.err)
		}
		if err != nil && !strings.Contains(err.Error(), tcase.err) {
			t.Errorf("Parse invalid query(%q), got: %v, want: %s...", tcase.input, err, tcase.err)
		}
	}
}

func TestCaseSensitivity(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "create table A (\n\t`B` int\n)",
		output: "create table A (\n\tB int\n)",
	}, {
		input:  "create index b on A (col1 desc)",
		output: "alter table A add index b (col1 desc)",
	}, {
		input:  "alter table A foo",
		output: "alter table A",
	}, {
		input:  "alter table A convert",
		output: "alter table A",
	}, {
		// View names get lower-cased.
		input:  "alter view A foo",
		output: "alter table a",
	}, {
		input:  "alter table A rename to B",
		output: "rename table A to B",
	}, {
		input: "rename table A to B",
	}, {
		input:  "drop table B",
		output: "drop table B",
	}, {
		input:  "drop table if exists B",
		output: "drop table if exists B",
	}, {
		input:  "drop index b on A",
		output: "alter table A",
	}, {
		input: "select a from B",
	}, {
		input: "select A as B from C",
	}, {
		input: "select B.* from c",
	}, {
		input: "select B.A from c",
	}, {
		input: "select * from B as C",
	}, {
		input: "select * from A.B",
	}, {
		input: "update A set b = 1",
	}, {
		input: "update A.B set b = 1",
	}, {
		input: "select A() from b",
	}, {
		input: "select A(B, C) from b",
	}, {
		input: "select A(distinct B, C) from b",
	}, {
		// IF is an exception. It's always lower-cased.
		input:  "select IF(B, C) from b",
		output: "select if(B, C) from b",
	}, {
		input: "select * from b use index (A)",
	}, {
		input: "insert into A(A, B) values (1, 2)",
	}, {
		input:  "CREATE TABLE A (\n\t`A` int\n)",
		output: "create table A (\n\tA int\n)",
	}, {
		input:  "create view A as select * from b",
		output: "create view a as select * from b",
	}, {
		input:  "alter view A",
		output: "alter table a",
	}, {
		input:  "drop view A",
		output: "drop table a",
	}, {
		input:  "drop view if exists A",
		output: "drop table if exists a",
	}, {
		input:  "select /* lock in SHARE MODE */ 1 from t lock in SHARE MODE",
		output: "select /* lock in SHARE MODE */ 1 from t lock in share mode",
	}, {
		input:  "select next VALUE from t",
		output: "select next 1 values from t",
	}, {
		input: "select /* use */ 1 from t1 use index (A) where b = 1",
	}}
	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}
}

func TestKeywords(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select current_timestamp",
		output: "select current_timestamp() from dual",
	}, {
		input: "update t set a = current_timestamp()",
	}, {
		input: "update t set a = current_timestamp(5)",
	}, {
		input:  "select a, current_date from t",
		output: "select a, current_date() from t",
	}, {
		input:  "insert into t(a, b) values (current_date, current_date())",
		output: "insert into t(a, b) values (current_date(), current_date())",
	}, {
		input: "select * from t where a > utc_timestmp()",
	}, {
		input: "select * from t where a > utc_timestamp(4)",
	}, {
		input:  "update t set b = utc_timestamp + 5",
		output: "update t set b = utc_timestamp() + 5",
	}, {
		input:  "select utc_time, utc_date, utc_time(6)",
		output: "select utc_time(), utc_date(), utc_time(6) from dual",
	}, {
		input:  "select 1 from dual where localtime > utc_time",
		output: "select 1 from dual where localtime() > utc_time()",
	}, {
		input:  "select 1 from dual where localtime(2) > utc_time(1)",
		output: "select 1 from dual where localtime(2) > utc_time(1)",
	}, {
		input:  "update t set a = localtimestamp(), b = utc_timestamp",
		output: "update t set a = localtimestamp(), b = utc_timestamp()",
	}, {
		input:  "update t set a = localtimestamp(10), b = utc_timestamp(13)",
		output: "update t set a = localtimestamp(10), b = utc_timestamp(13)",
	}, {
		input: "insert into t(a) values (unix_timestamp)",
	}, {
		input: "select replace(a, 'foo', 'bar') from t",
	}, {
		input: "update t set a = replace('1234', '2', '1')",
	}, {
		input: "insert into t(a, b) values ('foo', 'bar') on duplicate key update a = replace(hex('foo'), 'f', 'b')",
	}, {
		input: "update t set a = left('1234', 3)",
	}, {
		input: "select left(a, 5) from t",
	}, {
		input: "update t set d = adddate(date('2003-12-31 01:02:03'), interval 5 days)",
	}, {
		input: "insert into t(a, b) values (left('foo', 1), 'b')",
	}, {
		input: "insert /* qualified function */ into t(a, b) values (test.PI(), 'b')",
	}, {
		input:  "select /* keyword in qualified id */ * from t join z on t.key = z.key",
		output: "select /* keyword in qualified id */ * from t join z on t.`key` = z.`key`",
	}, {
		input:  "select /* non-reserved keywords as unqualified cols */ date, view, offset from t",
		output: "select /* non-reserved keywords as unqualified cols */ `date`, `view`, `offset` from t",
	}, {
		input:  "select /* share and mode as cols */ share, mode from t where share = 'foo'",
		output: "select /* share and mode as cols */ `share`, `mode` from t where `share` = 'foo'",
	}, {
		input:  "select /* unused keywords as cols */ `write`, varying from t where trailing = 'foo'",
		output: "select /* unused keywords as cols */ `write`, `varying` from t where `trailing` = 'foo'",
	}, {
		input:  "select status from t",
		output: "select `status` from t",
	}, {
		input:  "select Status from t",
		output: "select `Status` from t",
	}, {
		input:  "select variables from t",
		output: "select `variables` from t",
	}, {
		input:  "select Variables from t",
		output: "select `Variables` from t",
	}}

	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}
}

func TestConvert(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select cast('abc' as date) from t",
		output: "select convert('abc', date) from t",
	}, {
		input: "select convert('abc', binary(4)) from t",
	}, {
		input: "select convert('abc', binary) from t",
	}, {
		input: "select convert('abc', char character set binary) from t",
	}, {
		input: "select convert('abc', char(4) ascii) from t",
	}, {
		input: "select convert('abc', char unicode) from t",
	}, {
		input: "select convert('abc', char(4)) from t",
	}, {
		input: "select convert('abc', char) from t",
	}, {
		input: "select convert('abc', nchar(4)) from t",
	}, {
		input: "select convert('abc', nchar) from t",
	}, {
		input: "select convert('abc', signed) from t",
	}, {
		input:  "select convert('abc', signed integer) from t",
		output: "select convert('abc', signed) from t",
	}, {
		input: "select convert('abc', unsigned) from t",
	}, {
		input:  "select convert('abc', unsigned integer) from t",
		output: "select convert('abc', unsigned) from t",
	}, {
		input: "select convert('abc', decimal(3, 4)) from t",
	}, {
		input: "select convert('abc', decimal(4)) from t",
	}, {
		input: "select convert('abc', decimal) from t",
	}, {
		input: "select convert('abc', date) from t",
	}, {
		input: "select convert('abc', time(4)) from t",
	}, {
		input: "select convert('abc', time) from t",
	}, {
		input: "select convert('abc', datetime(9)) from t",
	}, {
		input: "select convert('abc', datetime) from t",
	}, {
		input: "select convert('abc', json) from t",
	}, {
		input: "select convert('abc' using ascii) from t",
	}}

	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}

	invalidSQL := []struct {
		input  string
		output string
	}{{
		input:  "select convert('abc' as date) from t",
		output: "syntax error at position 24 near 'as'",
	}, {
		input:  "select convert from t",
		output: "syntax error at position 20 near 'from'",
	}, {
		input:  "select cast('foo', decimal) from t",
		output: "syntax error at position 19",
	}, {
		input:  "select convert('abc', datetime(4+9)) from t",
		output: "syntax error at position 34",
	}, {
		input:  "select convert('abc', decimal(4+9)) from t",
		output: "syntax error at position 33",
	}, {
		input:  "/* a comment */",
		output: "empty statement",
	}, {
		input:  "set transaction isolation level 12345",
		output: "syntax error at position 38 near '12345'",
	}}

	for _, tcase := range invalidSQL {
		_, err := Parse(tcase.input)
		if err == nil || err.Error() != tcase.output {
			t.Errorf("%s: %v, want %s", tcase.input, err, tcase.output)
		}
	}
}

func TestSelectInto(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select * from t order by name limit 100 into outfile s3 'out_file_name'",
		output: "select * from t order by `name` asc limit 100 into outfile s3 'out_file_name'",
	}, {
		input: "select * from t into dumpfile 'out_file_name'",
	}, {
		input: "select * from t into outfile 'out_file_name' character set binary fields terminated by 'term' optionally enclosed by 'c' escaped by 'e' lines starting by 'a' terminated by '\n'",
	}, {
		input: "select * from t into outfile s3 'out_file_name' character set binary format csv header fields terminated by 'term' optionally enclosed by 'c' escaped by 'e' lines starting by 'a' terminated by '\n' manifest on overwrite off",
	}, {
		input:  "select * from (select * from t union select * from t2) as t3 where t3.name in (select col from t4) into outfile s3 'out_file_name'",
		output: "select * from (select * from t union select * from t2) as t3 where t3.`name` in (select col from t4) into outfile s3 'out_file_name'",
	}, {
		// Invalid queries but these are parsed and errors caught in planbuilder
		input: "select * from t limit 100 into outfile s3 'out_file_name' union select * from t2",
	}, {
		input: "select * from (select * from t into outfile s3 'inner_outfile') as t2 into outfile s3 'out_file_name'",
	}}

	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		assert.Equal(t, tcase.output, out)
	}

	invalidSQL := []struct {
		input  string
		output string
	}{{
		input:  "select convert('abc' as date) from t",
		output: "syntax error at position 24 near 'as'",
	}, {
		input:  "set transaction isolation level 12345",
		output: "syntax error at position 38 near '12345'",
	}}

	for _, tcase := range invalidSQL {
		_, err := Parse(tcase.input)
		if err == nil || err.Error() != tcase.output {
			t.Errorf("%s: %v, want %s", tcase.input, err, tcase.output)
		}
	}
}

func TestPositionedErr(t *testing.T) {
	invalidSQL := []struct {
		input  string
		output PositionedErr
	}{{
		input:  "select convert('abc' as date) from t",
		output: PositionedErr{"syntax error", 24, []byte("as")},
	}, {
		input:  "select convert from t",
		output: PositionedErr{"syntax error", 20, []byte("from")},
	}, {
		input:  "select cast('foo', decimal) from t",
		output: PositionedErr{"syntax error", 19, nil},
	}, {
		input:  "select convert('abc', datetime(4+9)) from t",
		output: PositionedErr{"syntax error", 34, nil},
	}, {
		input:  "select convert('abc', decimal(4+9)) from t",
		output: PositionedErr{"syntax error", 33, nil},
	}, {
		input:  "set transaction isolation level 12345",
		output: PositionedErr{"syntax error", 38, []byte("12345")},
	}, {
		input:  "select * from a left join b",
		output: PositionedErr{"syntax error", 28, nil},
	}, {
		input:  "select a from (select * from tbl)",
		output: PositionedErr{"syntax error", 34, nil},
	}}

	for _, tcase := range invalidSQL {
		tkn := NewStringTokenizer(tcase.input)
		_, err := ParseNext(tkn)

		if posErr, ok := err.(PositionedErr); !ok {
			t.Errorf("%s: %v expected PositionedErr, got (%T) %v", tcase.input, err, err, tcase.output)
		} else if posErr.Pos != tcase.output.Pos || !bytes.Equal(posErr.Near, tcase.output.Near) || err.Error() != tcase.output.Error() {
			t.Errorf("%s: %v, want: %v", tcase.input, err, tcase.output)
		}
	}
}

func TestSubStr(t *testing.T) {

	validSQL := []struct {
		input  string
		output string
	}{{
		input: `select substr('foobar', 1) from t`,
	}, {
		input: "select substr(a, 1, 6) from t",
	}, {
		input:  "select substring(a, 1) from t",
		output: "select substr(a, 1) from t",
	}, {
		input:  "select substring(a, 1, 6) from t",
		output: "select substr(a, 1, 6) from t",
	}, {
		input:  "select substr(a from 1 for 6) from t",
		output: "select substr(a, 1, 6) from t",
	}, {
		input:  "select substring(a from 1 for 6) from t",
		output: "select substr(a, 1, 6) from t",
	}, {
		input:  `select substr("foo" from 1 for 2) from t`,
		output: `select substr('foo', 1, 2) from t`,
	}, {
		input:  `select substring("foo", 1, 2) from t`,
		output: `select substr('foo', 1, 2) from t`,
	}, {
		input:  `select substr(substr("foo" from 1 for 2), 1, 2) from t`,
		output: `select substr(substr('foo', 1, 2), 1, 2) from t`,
	}, {
		input:  `select substr(substring("foo", 1, 2), 3, 4) from t`,
		output: `select substr(substr('foo', 1, 2), 3, 4) from t`,
	}, {
		input:  `select substring(substr("foo", 1), 2) from t`,
		output: `select substr(substr('foo', 1), 2) from t`,
	}}

	for _, tcase := range validSQL {
		if tcase.output == "" {
			tcase.output = tcase.input
		}
		tree, err := Parse(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		out := String(tree)
		if out != tcase.output {
			t.Errorf("out: %s, want %s", out, tcase.output)
		}
	}
}

func TestLoadData(t *testing.T) {
	validSQL := []string{
		"load data from s3 'x.txt'",
		"load data from s3 manifest 'x.txt'",
		"load data from s3 file 'x.txt'",
		"load data infile 'x.txt' into table 'c'",
		"load data from s3 'x.txt' into table x"}
	for _, tcase := range validSQL {
		_, err := Parse(tcase)
		require.NoError(t, err)
	}
}

func TestCreateTable(t *testing.T) {
	validSQL := []string{
		// test all the data types and options
		"create table t (\n" +
			"	col_bit bit,\n" +
			"	col_tinyint tinyint auto_increment,\n" +
			"	col_tinyint3 tinyint(3) unsigned,\n" +
			"	col_smallint smallint,\n" +
			"	col_smallint4 smallint(4) zerofill,\n" +
			"	col_mediumint mediumint,\n" +
			"	col_mediumint5 mediumint(5) unsigned not null,\n" +
			"	col_int int,\n" +
			"	col_int10 int(10) not null,\n" +
			"	col_integer integer comment 'this is an integer',\n" +
			"	col_bigint bigint,\n" +
			"	col_bigint10 bigint(10) zerofill not null default 10,\n" +
			"	col_real real,\n" +
			"	col_real2 real(1,2) not null default 1.23,\n" +
			"	col_double double,\n" +
			"	col_double2 double(3,4) not null default 1.23,\n" +
			"	col_float float,\n" +
			"	col_float2 float(3,4) not null default 1.23,\n" +
			"	col_decimal decimal,\n" +
			"	col_decimal2 decimal(2),\n" +
			"	col_decimal3 decimal(2,3),\n" +
			"	col_numeric numeric,\n" +
			"	col_numeric2 numeric(2),\n" +
			"	col_numeric3 numeric(2,3),\n" +
			"	col_date date,\n" +
			"	col_time time,\n" +
			"	col_timestamp timestamp,\n" +
			"	col_datetime datetime,\n" +
			"	col_year year,\n" +
			"	col_char char,\n" +
			"	col_char2 char(2),\n" +
			"	col_char3 char(3) character set ascii,\n" +
			"	col_char4 char(4) character set ascii collate ascii_bin,\n" +
			"	col_varchar varchar,\n" +
			"	col_varchar2 varchar(2),\n" +
			"	col_varchar3 varchar(3) character set ascii,\n" +
			"	col_varchar4 varchar(4) character set ascii collate ascii_bin,\n" +
			"	col_binary binary,\n" +
			"	col_varbinary varbinary(10),\n" +
			"	col_tinyblob tinyblob,\n" +
			"	col_blob blob,\n" +
			"	col_mediumblob mediumblob,\n" +
			"	col_longblob longblob,\n" +
			"	col_tinytext tinytext,\n" +
			"	col_text text,\n" +
			"	col_mediumtext mediumtext,\n" +
			"	col_longtext longtext,\n" +
			"	col_text text character set ascii collate ascii_bin,\n" +
			"	col_json json,\n" +
			"	col_enum enum('a', 'b', 'c', 'd'),\n" +
			"	col_enum2 enum('a', 'b', 'c', 'd') character set ascii,\n" +
			"	col_enum3 enum('a', 'b', 'c', 'd') collate ascii_bin,\n" +
			"	col_enum4 enum('a', 'b', 'c', 'd') character set ascii collate ascii_bin,\n" +
			"	col_set set('a', 'b', 'c', 'd'),\n" +
			"	col_set2 set('a', 'b', 'c', 'd') character set ascii,\n" +
			"	col_set3 set('a', 'b', 'c', 'd') collate ascii_bin,\n" +
			"	col_set4 set('a', 'b', 'c', 'd') character set ascii collate ascii_bin,\n" +
			"	col_geometry1 geometry,\n" +
			"	col_geometry2 geometry not null,\n" +
			"	col_point1 point,\n" +
			"	col_point2 point not null,\n" +
			"	col_linestring1 linestring,\n" +
			"	col_linestring2 linestring not null,\n" +
			"	col_polygon1 polygon,\n" +
			"	col_polygon2 polygon not null,\n" +
			"	col_geometrycollection1 geometrycollection,\n" +
			"	col_geometrycollection2 geometrycollection not null,\n" +
			"	col_multipoint1 multipoint,\n" +
			"	col_multipoint2 multipoint not null,\n" +
			"	col_multilinestring1 multilinestring,\n" +
			"	col_multilinestring2 multilinestring not null,\n" +
			"	col_multipolygon1 multipolygon,\n" +
			"	col_multipolygon2 multipolygon not null\n" +
			")",

		// test defining indexes separately
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	full_name varchar,\n" +
			"	geom point not null,\n" +
			"	status_nonkeyword varchar,\n" +
			"	primary key (id),\n" +
			"	spatial key geom (geom),\n" +
			"	fulltext key fts (full_name),\n" +
			"	unique key by_username (username),\n" +
			"	unique key by_username2 (username),\n" +
			"	unique index by_username3 (username),\n" +
			"	index by_status (status_nonkeyword),\n" +
			"	key by_full_name (full_name)\n" +
			")",

		// test that indexes support USING <id>
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	full_name varchar,\n" +
			"	status_nonkeyword varchar,\n" +
			"	primary key (id) using BTREE,\n" +
			"	unique key by_username (username) using HASH,\n" +
			"	unique key by_username2 (username) using OTHER,\n" +
			"	unique index by_username3 (username) using XYZ,\n" +
			"	index by_status (status_nonkeyword) using PDQ,\n" +
			"	key by_full_name (full_name) using OTHER\n" +
			")",
		// test other index options
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	primary key (id) comment 'hi',\n" +
			"	unique key by_username (username) key_block_size 8,\n" +
			"	unique index by_username4 (username) comment 'hi' using BTREE,\n" +
			"	unique index by_username4 (username) using BTREE key_block_size 4 comment 'hi'\n" +
			")",

		// multi-column indexes
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	email varchar,\n" +
			"	full_name varchar,\n" +
			"	a int,\n" +
			"	b int,\n" +
			"	c int,\n" +
			"	primary key (id, username),\n" +
			"	unique key by_abc (a, b, c),\n" +
			"	unique key (a, b, c),\n" +
			"	key by_email (email(10), username)\n" +
			")",

		// foreign keys
		"create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	k int,\n" +
			"	Z int,\n" +
			"	primary key (id, username),\n" +
			"	key by_email (email(10), username),\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b),\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b) on delete restrict,\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b) on delete no action,\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b) on delete cascade on update set default,\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b) on delete set default on update set null,\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b) on delete set null on update restrict,\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b) on update no action,\n" +
			"	constraint second_ibfk_1 foreign key (k, j) references simple (a, b) on update cascade\n" +
			")",

		// table options
		"create table t (\n" +
			"	id int auto_increment\n" +
			") engine InnoDB,\n" +
			"  auto_increment 123,\n" +
			"  avg_row_length 1,\n" +
			"  default character set utf8mb4,\n" +
			"  character set latin1,\n" +
			"  checksum 0,\n" +
			"  default collate binary,\n" +
			"  collate ascii_bin,\n" +
			"  comment 'this is a comment',\n" +
			"  compression 'zlib',\n" +
			"  connection 'connect_string',\n" +
			"  data directory 'absolute path to directory',\n" +
			"  delay_key_write 1,\n" +
			"  encryption 'n',\n" +
			"  index directory 'absolute path to directory',\n" +
			"  insert_method no,\n" +
			"  key_block_size 1024,\n" +
			"  max_rows 100,\n" +
			"  min_rows 10,\n" +
			"  pack_keys 0,\n" +
			"  password 'sekret',\n" +
			"  row_format default,\n" +
			"  stats_auto_recalc default,\n" +
			"  stats_persistent 0,\n" +
			"  stats_sample_pages 1,\n" +
			"  tablespace tablespace_name storage disk,\n" +
			"  tablespace tablespace_name\n",

		// boolean columns
		"create table t (\n" +
			"	bi bigint not null primary key,\n" +
			"	b1 bool not null,\n" +
			"	b2 boolean\n" +
			")",
	}
	for _, sql := range validSQL {
		sql = strings.TrimSpace(sql)
		tree, err := ParseStrictDDL(sql)
		if err != nil {
			t.Errorf("input: %s, err: %v", sql, err)
			continue
		}
		got := String(tree.(*CreateTable))
		assert.True(t, tree.(*CreateTable).FullyParsed)
		if sql != got {
			t.Errorf("want:\n%s\ngot:\n%s", sql, got)
		}
	}

	sql := "create table t garbage"
	tree, err := Parse(sql)
	if err != nil {
		t.Errorf("input: %s, err: %v", sql, err)
	}
	assert.True(t, !tree.(*CreateTable).FullyParsed)

	tree, err = ParseStrictDDL(sql)
	if tree != nil || err == nil {
		t.Errorf("ParseStrictDDL unexpectedly accepted input %s", sql)
	}

	testCases := []struct {
		input  string
		output string
	}{{
		// test key_block_size
		input: "create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	unique key by_username (username) key_block_size 8,\n" +
			"	unique key by_username2 (username) key_block_size=8,\n" +
			"	unique by_username3 (username) key_block_size = 4\n" +
			")",
		output: "create table t (\n" +
			"	id int auto_increment,\n" +
			"	username varchar,\n" +
			"	unique key by_username (username) key_block_size 8,\n" +
			"	unique key by_username2 (username) key_block_size 8,\n" +
			"	unique key by_username3 (username) key_block_size 4\n" +
			")",
	}, {
		// test defaults
		input: "create table t (\n" +
			"	i1 int default 1,\n" +
			"	i2 int default null,\n" +
			"	f1 float default 1.23,\n" +
			"	s1 varchar default 'c',\n" +
			"	s2 varchar default 'this is a string',\n" +
			"	s3 varchar default null,\n" +
			"	s4 timestamp default current_timestamp,\n" +
			"	s5 bit(1) default B'0'\n" +
			")",
		output: "create table t (\n" +
			"	i1 int default 1,\n" +
			"	i2 int default null,\n" +
			"	f1 float default 1.23,\n" +
			"	s1 varchar default 'c',\n" +
			"	s2 varchar default 'this is a string',\n" +
			"	`s3` varchar default null,\n" +
			"	s4 timestamp default current_timestamp(),\n" +
			"	s5 bit(1) default B'0'\n" +
			")",
	}, {
		// test non_reserved word in column name
		input: "create table t (\n" +
			"	repair int\n" +
			")",
		output: "create table t (\n" +
			"	`repair` int\n" +
			")",
	}, {
		// test key field options
		input: "create table t (\n" +
			"	id int auto_increment primary key,\n" +
			"	username varchar unique key,\n" +
			"	email varchar unique,\n" +
			"	full_name varchar key,\n" +
			"	time1 timestamp on update current_timestamp,\n" +
			"	time2 timestamp default current_timestamp on update current_timestamp\n" +
			")",
		output: "create table t (\n" +
			"	id int auto_increment primary key,\n" +
			"	username varchar unique key,\n" +
			"	email varchar unique,\n" +
			"	full_name varchar key,\n" +
			"	time1 timestamp on update current_timestamp(),\n" +
			"	time2 timestamp default current_timestamp() on update current_timestamp()\n" +
			")",
	}, {
		// test current_timestamp with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default current_timestamp,\n" +
			"	time2 timestamp default current_timestamp(),\n" +
			"	time3 timestamp default current_timestamp on update current_timestamp,\n" +
			"	time4 timestamp default current_timestamp() on update current_timestamp(),\n" +
			"	time5 timestamp(3) default current_timestamp(3) on update current_timestamp(3)\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default current_timestamp(),\n" +
			"	time2 timestamp default current_timestamp(),\n" +
			"	time3 timestamp default current_timestamp() on update current_timestamp(),\n" +
			"	time4 timestamp default current_timestamp() on update current_timestamp(),\n" +
			"	time5 timestamp(3) default current_timestamp(3) on update current_timestamp(3)\n" +
			")",
	}, {
		// test utc_timestamp with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default utc_timestamp,\n" +
			"	time2 timestamp default utc_timestamp(),\n" +
			"	time3 timestamp default utc_timestamp on update utc_timestamp,\n" +
			"	time4 timestamp default utc_timestamp() on update utc_timestamp(),\n" +
			"	time5 timestamp(4) default utc_timestamp(4) on update utc_timestamp(4)\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default utc_timestamp(),\n" +
			"	time2 timestamp default utc_timestamp(),\n" +
			"	time3 timestamp default utc_timestamp() on update utc_timestamp(),\n" +
			"	time4 timestamp default utc_timestamp() on update utc_timestamp(),\n" +
			"	time5 timestamp(4) default utc_timestamp(4) on update utc_timestamp(4)\n" +
			")",
	}, {
		// test utc_time with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default utc_time,\n" +
			"	time2 timestamp default utc_time(),\n" +
			"	time3 timestamp default utc_time on update utc_time,\n" +
			"	time4 timestamp default utc_time() on update utc_time(),\n" +
			"	time5 timestamp(5) default utc_time(5) on update utc_time(5)\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default utc_time(),\n" +
			"	time2 timestamp default utc_time(),\n" +
			"	time3 timestamp default utc_time() on update utc_time(),\n" +
			"	time4 timestamp default utc_time() on update utc_time(),\n" +
			"	time5 timestamp(5) default utc_time(5) on update utc_time(5)\n" +
			")",
	}, {
		// test utc_date with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default utc_date,\n" +
			"	time2 timestamp default utc_date(),\n" +
			"	time3 timestamp default utc_date on update utc_date,\n" +
			"	time4 timestamp default utc_date() on update utc_date()\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default utc_date(),\n" +
			"	time2 timestamp default utc_date(),\n" +
			"	time3 timestamp default utc_date() on update utc_date(),\n" +
			"	time4 timestamp default utc_date() on update utc_date()\n" +
			")",
	}, {
		// test localtime with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default localtime,\n" +
			"	time2 timestamp default localtime(),\n" +
			"	time3 timestamp default localtime on update localtime,\n" +
			"	time4 timestamp default localtime() on update localtime(),\n" +
			"	time5 timestamp(6) default localtime(6) on update localtime(6)\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default localtime(),\n" +
			"	time2 timestamp default localtime(),\n" +
			"	time3 timestamp default localtime() on update localtime(),\n" +
			"	time4 timestamp default localtime() on update localtime(),\n" +
			"	time5 timestamp(6) default localtime(6) on update localtime(6)\n" +
			")",
	}, {
		// test localtimestamp with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default localtimestamp,\n" +
			"	time2 timestamp default localtimestamp(),\n" +
			"	time3 timestamp default localtimestamp on update localtimestamp,\n" +
			"	time4 timestamp default localtimestamp() on update localtimestamp(),\n" +
			"	time5 timestamp(1) default localtimestamp(1) on update localtimestamp(1)\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default localtimestamp(),\n" +
			"	time2 timestamp default localtimestamp(),\n" +
			"	time3 timestamp default localtimestamp() on update localtimestamp(),\n" +
			"	time4 timestamp default localtimestamp() on update localtimestamp(),\n" +
			"	time5 timestamp(1) default localtimestamp(1) on update localtimestamp(1)\n" +
			")",
	}, {
		// test current_date with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default current_date,\n" +
			"	time2 timestamp default current_date(),\n" +
			"	time3 timestamp default current_date on update current_date,\n" +
			"	time4 timestamp default current_date() on update current_date()\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default current_date(),\n" +
			"	time2 timestamp default current_date(),\n" +
			"	time3 timestamp default current_date() on update current_date(),\n" +
			"	time4 timestamp default current_date() on update current_date()\n" +
			")",
	}, {
		// test current_time with and without ()
		input: "create table t (\n" +
			"	time1 timestamp default current_time,\n" +
			"	time2 timestamp default current_time(),\n" +
			"	time3 timestamp default current_time on update current_time,\n" +
			"	time4 timestamp default current_time() on update current_time(),\n" +
			"	time5 timestamp(2) default current_time(2) on update current_time(2)\n" +
			")",
		output: "create table t (\n" +
			"	time1 timestamp default current_time(),\n" +
			"	time2 timestamp default current_time(),\n" +
			"	time3 timestamp default current_time() on update current_time(),\n" +
			"	time4 timestamp default current_time() on update current_time(),\n" +
			"	time5 timestamp(2) default current_time(2) on update current_time(2)\n" +
			")",
	},
	}
	for _, tcase := range testCases {
		tree, err := ParseStrictDDL(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		assert.True(t, tree.(*CreateTable).FullyParsed)
		if got, want := String(tree.(*CreateTable)), tcase.output; got != want {
			t.Errorf("Parse(%s):\n%s, want\n%s", tcase.input, got, want)
		}
	}
}

func TestCreateTableLike(t *testing.T) {
	normal := "create table a like b"
	testCases := []struct {
		input  string
		output string
	}{
		{
			"create table a like b",
			normal,
		},
		{
			"create table a (like b)",
			normal,
		},
		{
			"create table ks.a like unsharded_ks.b",
			"create table ks.a like unsharded_ks.b",
		},
	}
	for _, tcase := range testCases {
		tree, err := ParseStrictDDL(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		assert.True(t, tree.(*CreateTable).FullyParsed)
		if got, want := String(tree.(*CreateTable)), tcase.output; got != want {
			t.Errorf("Parse(%s):\n%s, want\n%s", tcase.input, got, want)
		}
	}
}

func TestCreateTableEscaped(t *testing.T) {
	testCases := []struct {
		input  string
		output string
	}{{
		input: "create table `a`(`id` int, primary key(`id`))",
		output: "create table a (\n" +
			"\tid int,\n" +
			"\tprimary key (id)\n" +
			")",
	}, {
		input: "create table `insert`(`update` int, primary key(`delete`))",
		output: "create table `insert` (\n" +
			"\t`update` int,\n" +
			"\tprimary key (`delete`)\n" +
			")",
	}}
	for _, tcase := range testCases {
		tree, err := ParseStrictDDL(tcase.input)
		if err != nil {
			t.Errorf("input: %s, err: %v", tcase.input, err)
			continue
		}
		if got, want := String(tree.(*CreateTable)), tcase.output; got != want {
			t.Errorf("Parse(%s):\n%s, want\n%s", tcase.input, got, want)
		}
	}
}

var (
	invalidSQL = []struct {
		input        string
		output       string
		excludeMulti bool // Don't use in the ParseNext multi-statement parsing tests.
	}{{
		input:  "select : from t",
		output: "syntax error at position 9 near ':'",
	}, {
		input:  "select 0xH from t",
		output: "syntax error at position 10 near '0x'",
	}, {
		input:  "select x'78 from t",
		output: "syntax error at position 12 near '78'",
	}, {
		input:  "select x'777' from t",
		output: "syntax error at position 14 near '777'",
	}, {
		input:  "select * from t where :1 = 2",
		output: "syntax error at position 24 near ':'",
	}, {
		input:  "select * from t where :. = 2",
		output: "syntax error at position 24 near ':'",
	}, {
		input:  "select * from t where ::1 = 2",
		output: "syntax error at position 25 near '::'",
	}, {
		input:  "select * from t where ::. = 2",
		output: "syntax error at position 25 near '::'",
	}, {
		input:  "update a set c = values(1)",
		output: "syntax error at position 26 near '1'",
	}, {
		input: "select(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(",
		output: "max nesting level reached at position 406",
	}, {
		input: "select(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(" +
			"F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F(F" +
			"(F(F(F(F(F(F(F(F(F(F(F(",
		output: "syntax error at position 404",
	}, {
		// This construct is considered invalid due to a grammar conflict.
		input:  "insert into a select * from b join c on duplicate key update d=e",
		output: "syntax error at position 54 near 'key'",
	}, {
		input:  "select * from a left join b",
		output: "syntax error at position 28",
	}, {
		input:  "select * from a natural join b on c = d",
		output: "syntax error at position 34 near 'on'",
	}, {
		input:  "select * from a natural join b using (c)",
		output: "syntax error at position 37 near 'using'",
	}, {
		input:  "select next id from a",
		output: "expecting value after next at position 15 near 'id'",
	}, {
		input:  "select next 1+1 values from a",
		output: "syntax error at position 15",
	}, {
		input:  "insert into a values (select * from b)",
		output: "syntax error at position 29 near 'select'",
	}, {
		input:  "select database",
		output: "syntax error at position 16",
	}, {
		input:  "select mod from t",
		output: "syntax error at position 16 near 'from'",
	}, {
		input:  "select 1 from t where div 5",
		output: "syntax error at position 26 near 'div'",
	}, {
		input:  "select 1 from t where binary",
		output: "syntax error at position 29",
	}, {
		input:  "select match(a1, a2) against ('foo' in boolean mode with query expansion) from t",
		output: "syntax error at position 57 near 'with'",
	}, {
		input:  "select /* reserved keyword as unqualified column */ * from t where key = 'test'",
		output: "syntax error at position 71 near 'key'",
	}, {
		input:  "select /* vitess-reserved keyword as unqualified column */ * from t where escape = 'test'",
		output: "syntax error at position 81 near 'escape'",
	}, {
		input:  "select * from t where id = ((select a from t1 union select b from t2) order by a limit 1)",
		output: "syntax error at position 76 near 'order'",
	}, {
		input:  "select /* straight_join using */ 1 from t1 straight_join t2 using (a)",
		output: "syntax error at position 66 near 'using'",
	}, {
		input:        "select 'aa",
		output:       "syntax error at position 11 near 'aa'",
		excludeMulti: true,
	}, {
		input:        "select 'aa\\",
		output:       "syntax error at position 12 near 'aa'",
		excludeMulti: true,
	}, {
		input:        "select /* aa",
		output:       "syntax error at position 13 near '/* aa'",
		excludeMulti: true,
	}, {
		// non_reserved keywords are currently not permitted everywhere
		input:        "create database repair",
		output:       "syntax error at position 23 near 'repair'",
		excludeMulti: true,
	}}
)

func TestErrors(t *testing.T) {
	for _, tcase := range invalidSQL {
		t.Run(tcase.input, func(t *testing.T) {
			_, err := Parse(tcase.input)
			require.Error(t, err, tcase.output)
			require.Equal(t, err.Error(), tcase.output)
		})
	}
}

// TestSkipToEnd tests that the skip to end functionality
// does not skip past a ';'. If any tokens exist after that, Parse
// should return an error.
func TestSkipToEnd(t *testing.T) {
	testcases := []struct {
		input  string
		output string
	}{{
		// This is the case where the partial ddl will be reset
		// because of a premature ';'.
		input:  "create table a(id; select * from t",
		output: "syntax error at position 19",
	}, {
		// Partial DDL should get reset for valid DDLs also.
		input:  "create table a(id int); select * from t",
		output: "syntax error at position 31 near 'select'",
	}, {
		// Partial DDL does not get reset here. But we allow the
		// DDL only if there are no new tokens after skipping to end.
		input:  "create table a bb cc; select * from t",
		output: "extra characters encountered after end of DDL: 'select'",
	}, {
		// Test that we don't step at ';' inside strings.
		input:  "create table a bb 'a;'; select * from t",
		output: "extra characters encountered after end of DDL: 'select'",
	}}
	for _, tcase := range testcases {
		_, err := Parse(tcase.input)
		if err == nil || err.Error() != tcase.output {
			t.Errorf("%s: %v, want %s", tcase.input, err, tcase.output)
		}
	}
}

func TestParseDjangoQueries(t *testing.T) {

	file, err := os.Open("./test_queries/django_queries.txt")
	if err != nil {
		t.Errorf(" Error: %v", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {

		_, err := Parse(string(scanner.Text()))
		if err != nil {
			t.Error(scanner.Text())
			t.Errorf(" Error: %v", err)
		}
	}
}

// Benchmark run on 6/23/17, prior to improvements:
// BenchmarkParse1-4         100000             16334 ns/op
// BenchmarkParse2-4          30000             44121 ns/op

// Benchmark run on 9/3/18, comparing pooled parser performance.
//
// benchmark                     old ns/op     new ns/op     delta
// BenchmarkNormalize-4          2540          2533          -0.28%
// BenchmarkParse1-4             18269         13330         -27.03%
// BenchmarkParse2-4             46703         41255         -11.67%
// BenchmarkParse2Parallel-4     22246         20707         -6.92%
// BenchmarkParse3-4             4064743       4083135       +0.45%
//
// benchmark                     old allocs     new allocs     delta
// BenchmarkNormalize-4          27             27             +0.00%
// BenchmarkParse1-4             75             74             -1.33%
// BenchmarkParse2-4             264            263            -0.38%
// BenchmarkParse2Parallel-4     176            175            -0.57%
// BenchmarkParse3-4             360            361            +0.28%
//
// benchmark                     old bytes     new bytes     delta
// BenchmarkNormalize-4          821           821           +0.00%
// BenchmarkParse1-4             22776         2307          -89.87%
// BenchmarkParse2-4             28352         7881          -72.20%
// BenchmarkParse2Parallel-4     25712         5235          -79.64%
// BenchmarkParse3-4             6352082       6336307       -0.25%

const (
	sql1 = "select 'abcd', 20, 30.0, eid from a where 1=eid and name='3'"
	sql2 = "select aaaa, bbb, ccc, ddd, eeee, ffff, gggg, hhhh, iiii from tttt, ttt1, ttt3 where aaaa = bbbb and bbbb = cccc and dddd+1 = eeee group by fff, gggg having hhhh = iiii and iiii = jjjj order by kkkk, llll limit 3, 4"
)

func BenchmarkParse1(b *testing.B) {
	sql := sql1
	for i := 0; i < b.N; i++ {
		ast, err := Parse(sql)
		if err != nil {
			b.Fatal(err)
		}
		_ = String(ast)
	}
}

func BenchmarkParse2(b *testing.B) {
	sql := sql2
	for i := 0; i < b.N; i++ {
		ast, err := Parse(sql)
		if err != nil {
			b.Fatal(err)
		}
		_ = String(ast)
	}
}

func BenchmarkParse2Parallel(b *testing.B) {
	sql := sql2
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ast, err := Parse(sql)
			if err != nil {
				b.Fatal(err)
			}
			_ = ast
		}
	})
}

var benchQuery string

func init() {
	// benchQuerySize is the approximate size of the query.
	benchQuerySize := 1000000

	// Size of value is 1/10 size of query. Then we add
	// 10 such values to the where clause.
	var baseval bytes.Buffer
	for i := 0; i < benchQuerySize/100; i++ {
		// Add an escape character: This will force the upcoming
		// tokenizer improvement to still create a copy of the string.
		// Then we can see if avoiding the copy will be worth it.
		baseval.WriteString("\\'123456789")
	}

	var buf bytes.Buffer
	buf.WriteString("select a from t1 where v = 1")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&buf, " and v%d = \"%d%s\"", i, i, baseval.String())
	}
	benchQuery = buf.String()
}

func BenchmarkParse3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := Parse(benchQuery); err != nil {
			b.Fatal(err)
		}
	}
}
