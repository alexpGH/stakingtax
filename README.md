## Stakingtax
A reduced to the min tool for cosmos based chains to 
* check the local setup of the command line daemons (like e.g. gaiad, configuarble via *config.yaml*)
* use the daemon to retrieve all staking tax relevant information for a set of adresses (configurable via *addr.yaml*)
* convert the received and fee amount to a Fiat base (as required by tax authorities)

The result is stored in a csv file per address (*addr.csv*).

To use as low as possible bandwith, the count of retrieved transactions is stored in *addr_count.txt* and, togehter with the latest tx's blockheight, it is checked if the count still is valid (no pruning happened), such that we can only retrieve the not yet fetched transactions.
In case pruning happened, we increase the size of the backward-frame iteratively in order to re-calibrate the count again.

*As we only retrieve information, no private keys are required*


## Kudos

* The tax part is inspired by [stake.tax](https://stake.tax/), which does much more than I need, however reports wrong values for [restake](https://restake.app/cosmoshub) transactions.

* The set-up check was inspired by @gjermundbjaanes work on [making-ledger-work-on-restake](https://gjermund.tech/blog/making-ledger-work-on-restake/)


## Quick Start
### Download & compile
```
git clone https://github.com/alexpGH/stakingtax.git
cd stakingtax
go mod tidy
go build stakingtax.go

cp addrTemplate.yaml addr.yaml
```



### Display command line options
```
./stakingtax -help
```

### Run set-up check only 
```
./stakingtax -checkOnly
```
for all networks given in *config.yaml*, the chain registry information is retrieved and it is checked if a correct daemon version (one of the listed valid versions in the chain registry) is available and that the rpc node to be used is responsive. Information is stored in your network config (as you would do using e.g. `gaiad config`).

This is  useful even when you are not interested in tax information, but in checking your local setup (valid version of the daemon, responsive node) before sending some txs from command line.

Example output:
```
 Checking networks ===============================================================================
 Checking: fetchhub --------------------------------------------------------
 [OK] Your daemon version: v0.10.3 is up to date!
 Checking chain-Id in config
     [OK] Your chain-Id setting was correct: fetchhub-4
 Checking to have a responsive node
     Checking responsiveness of your node in config: https://rpc-fetchhub.fetch.ai:443
     [OK] -> node responded
 Checking: cosmoshub --------------------------------------------------------
 [OK] Your daemon version: v7.0.1 is up to date!
 Checking chain-Id in config
     [OK] Your chain-Id setting was correct: cosmoshub-4
 Checking to have a responsive node
     Checking responsiveness of your node in config: https://rpc-cosmoshub.blockapsis.com:443
     [OK] -> node responded
 [OK] checking networks ==========================================================================
 
 Check networks only done.

```
### The config file
The config file (default is config.yaml) allows to adapt the basic source of information under `networkBasics` (no adaption necessary),
followed by a list of networks you want to retrieve tax info for.

The tradePairs4Tax subblock allows to use one of currently two open access exchange APIs to convert from network denom to your Fiat base, e.g. in the fetch.ai example from FET -> BTC -> €.
Use as many pairs as necessary in your case.

`pageLimit` sets the page size used when retrieving messages. The setting should approximately match the number of expected messages (per address) for frequent syncing. 

Example: if you expect one tx per day and sync about once per week, 10 would be a good choice. Using e.g. 1000 would meant that you fetch the latest 1000 txs to actually get less than ten - a waste of bandwidth. The other way round: if you expect to get 10,000 messages and use a setting of 10 would  mean to bother the node 1000 times to collect all your messages while only sending 10 each time. 

`taxRelevantMessageTypes` lists all message types I found to be related to staking tax relevant transactions.

```
#config file for stakingtax
networksBasics:
  chainRegistry: https://raw.githubusercontent.com/cosmos/chain-registry
  chainExtraPath: /master/
  chainInfo: /chain.json

networks:
  - name: fetchhub
    denom: fet
    exponent: 18
    feedenom: afet
    tradePairs4Tax:       #tradePairs4Tax 
      endpoint: binance
      pairs:
        - FETBTC
        - BTCEUR
  - name: cosmoshub
    denom: atom
    exponent: 6  
    feedenom: uatom
    tradePairs4Tax:
      endpoint: cbpro
      pairs:
        - ATOM-EUR
  
query:
  pageLimit: 40
  
taxRelevantMessageTypes:
  - /cosmos.staking.v1beta1.MsgDelegate
  - /cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward
  - /cosmos.staking.v1beta1.MsgBeginRedelegate
  - /cosmos.authz.v1beta1.MsgExec
  - /cosmos.authz.v1beta1.MsgGrant

```
### Address file
The address file (default is addr.yaml) lists the addresses per network, for which staking tax relevant information should be fetched.
The pubKey is necessary in order to be able to check for who payed the tx fees. This is e.g. important for the restake approach: in this case, the validator pays the fees, so you can not set it off against the other tax liabilities (see *More details* for a detailed discussion).


```
#config file for staking tax: tax relevant addresses
#add an entry for each address
addresses:
  - chainName: fetchhub
    addrList:
      - addr: fetch...
        pubKey: yourPubKey


  - chainName: cosmoshub
    addrList:
      - addr: cosmos...
        pubKey: yourPubKey
```

### Full run
Assuming you have a config.yaml and addr.yam in place, run 

```
./stakingtax
```
As a result, you will get a addr.csv and addr_count.txt storing the tax relevant information and count of received transactions so far. The latter is used to only request as little as possible transactions in the next run (only from the page containing the new ones onward).

The output looks similar to
```
Checking networks ===============================================================================
Checking: fetchhub --------------------------------------------------------
[OK] Your daemon version: v0.10.3 is up to date!
Checking chain-Id in config
    [OK] Your chain-Id setting was correct: fetchhub-4
Checking to have a responsive node
    Checking responsiveness of your node in config: https://rpc-fetchhub.fetch.ai:443
    [OK] -> node responded
Checking: cosmoshub --------------------------------------------------------
[OK] Your daemon version: v7.0.1 is up to date!
Checking chain-Id in config
    [OK] Your chain-Id setting was correct: cosmoshub-4
Checking to have a responsive node
    Checking responsiveness of your node in config: https://rpc-cosmoshub.blockapsis.com:443
    [OK] -> node responded
[OK] checking networks ==========================================================================

Querying networks for txs =======================================================================
Querying fetchhub--------------------------------------------------------
   addr: fetch1XXX
   Checking totalCount hypothesis
      [I] txCountOld/totalCount: 64/64
   [OK] totalCount matches txCountOld and blockHeights match
      [I] using txCount/totalCount: 64/64
   [OK] nothing to do
Querying cosmoshub--------------------------------------------------------
   addr: cosmos1XXX
   Checking totalCount hypothesis
      [I] txCountOld/totalCount: 53/54
   [OK] totalCount matches txCountOld and blockHeights match
      [I] using txCount/totalCount: 53/54
   [I] querying page: 6/6 - this may take some time!
   Getting Fiat conversion for received and fee amounts
   [OK] done, txCount now: 54
[OK] Querying networks for txs ==================================================================

```
## More details (how does it work, what are the hypothesis)
### How we retrieve tax relevant messages
Under the hood, a
```
daemon query txs --events 'message.sender=addr' --page x --limit y 
```
is used to retrieve all transactions where the given address occurs as sender.

Internally, we then check for message types to sort out those relevant for staking tax (based on the list given in the config file). E.g. just sending or receiving tokens is not tax relevant.

Hypothesis:
You occur as sender in any tax relevant message. E.g. during delegation, withdrawDelegatorReward etc. your adress occurs as sender in the event log.

This also holds for the restake code (tx issued and payed by the validator), as your address sends tokens during restaking.

We then note the amount of retrieved tokens (this are tax relevant rewards) and payed fees (only when payment signature relates to your pubKey).

The only *problematic* case is the grant tx (MsgGrant). I only use grant in context with addresses I stake from for restaking (no other grants on these addresses), so I can retrieve all grant tx's and note the fees payed.
If this is not the case for your situation you could leave them out (by deleting the line in the config; the tx fees are typically negligible).

There is another caveat in relation to the grant transactions: they only show up under `--events 'message.sender=addr'` when you payed tx fees. This is ok, as we are not interested in 0 fees txs. The reason for this is that, if you don't pay any fees for the tx, you do not occur as sender in the event log.

I tried to overcome all this by fetching the grant transactions individually; however, the authz module uses the new emitTypedEvent and therefore adds extra " around the value fields (see [this discussion](https://github.com/cosmos/cosmos-sdk/issues/12592)), which can however not be parsed by the current valid daemon versions: 

```
daemon query txs --events 'message.sender="addr"' --page x --limit y 
```
can currenlty not be used.

### Low bandwith approach
As discussed above, we retrieve the txs as chunks (page & limit options of the query command). The stored counter is compared to the totalCount reported by the node. The last blockheight we had is compared against the blockheight of the tx the node sends us for this txCount. If everything matches, we are fine to go on fetching the  missing pages.

In case the totalCount and blockhight does not match (may be due to pruning), we scan backwards starting from going back 10 txs, then doubling the step every iteration (if possible), until we found the known last blockhight or are at txCount=0 - more has been pruned than we had stored locally. 

In the first case, we can read forward from the found txCount. In the latter case, a warning is given and you need to *connect to an archive node* in oder to fetch all txs. You can do this by setting the archive node's address in your chain's config like e.g. for cosmoshub
```
gaiad config node https://rpc-cosmoshub.blockapsis.com:443
```
As the stakingtax tool first checks if the given node in your config is responsive, your setting is preserved as long as the node is responsive.

## Disclaimer

This is my *first* golang project, written mainly for personal use, missing tests etc. Hints on how to improve (besides opting for test driven developement, which I did in the meantime) are welcome.

Double check the result for possibly missing transactions. There is no guarantee of completeness or accuracy. In no event will I be liable for you or anyone else for any decision made or action taken in reliance on data from retrieved by this code. 
## Contributing

Contribution is welcome. Possibilities are (non-exhaustive):
* Check the tx retrieval hypothesis, possibly report / extend about further tax relevant messages; especially better handling of grant txs
* Add further exchange APIs for Fiat conversion
* **hot**: I could not figure out how to suppress the header when appending to the csv file (therefore, each new block of txs gets its own header line)
* Add optional csv write after each page of received txs; this could be useful when initializing for a large amount of txs
* As it is my first golang project, hints on how to improve are welcome