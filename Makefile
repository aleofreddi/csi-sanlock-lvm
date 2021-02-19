.PHONY: %

all: build ;

%: %.deploy %.diskrpc %.driverd %.lvmctrld  %.proto ;

build.lvmctrld: build.proto
	$(MAKE) -C lvmctrld build

build.diskrpc: build.proto
	$(MAKE) -C diskrpc build

build.driverd: build.proto
	$(MAKE) -C driverd build

test.lvmctrld: test.proto
	$(MAKE) -C lvmctrld test

test.diskrpc: test.proto
	$(MAKE) -C diskrpc test

test.driverd: test.proto
	$(MAKE) -C driverd test

test: test.deploy test.diskrpc test.driverd test.lvmctrld test.proto
	cat */coverage.txt > coverage.txt

clean: clean.deploy clean.diskrpc clean.driverd clean.lvmctrld clean.proto
	$(RM) coverage.txt

%.deploy:
	$(MAKE) -C deploy $*

%.diskrpc:
	$(MAKE) -C diskrpc $*

%.driverd:
	$(MAKE) -C driverd $*

%.lvmctrld: 
	$(MAKE) -C lvmctrld $*

%.proto:
	$(MAKE) -C proto $*
