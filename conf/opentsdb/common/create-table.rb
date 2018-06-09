create 'tsdb-uid', \
  {NAME => 'id', COMPRESSION => 'NONE', BLOOMFILTER => 'ROW'}, \
  {NAME => 'name', COMPRESSION => 'NONE', BLOOMFILTER => 'ROW'}

create 'tsdb', \
  {NAME => 't', VERSIONS => 1, COMPRESSION => 'NONE', BLOOMFILTER => 'ROW'}

create 'tsdb-tree', \
  {NAME => 't', VERSIONS => 1, COMPRESSION => 'NONE', BLOOMFILTER => 'ROW'}

create 'tsdb-meta', \
  {NAME => 'name', COMPRESSION => 'NONE', BLOOMFILTER => 'ROW'}
