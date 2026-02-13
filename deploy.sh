#/bin/sh

#hostlist='bd4two.nrlptt.com'
hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com bd4vki.nrlptt.com  ah.nrlptt.com ptt.nrlptt.com bh1osw.nrlptt.com yz.hamoa.cn ham.73ham.com js.nrlptt.com bg1vif.nrlptt.com usa.nrlptt.com'

#hostlist='ptt.nrlptt.com bh1osw.nrlptt.com'

#hostlist='nrlptt.com bh4tdv.nrlptt.com ba1gm.nrlptt.com bd4vki.nrlptt.com  ah.nrlptt.com  yz.hamoa.cn ham.73ham.com '

#hostlist='ham.73ham.com'

#hostlist='nrlptt.com bd4vki.nrlptt.com ah.nrlptt.com'

#hostlist='js.nrlptt.com'

#hostlist='usa.nrlptt.com'

#hostlist='ba1gm.nrlptt.com'

#hostlist='nrlptt.com'

#hostlist='bh4tdv.nrlptt.com'

hostlist="js.nrlptt.com"

#scp nrlnanny root@192.168.35.40:nrlnanny/
 

time=`date "+%Y%m%d%H%M%S"`

#go build 

for i in $hostlist ; do     
echo "deploying to $i"
   scp nrlnanny root@$i:
   #scp play.html root@$i:/nrlnanny/
   #scp control.html root@$i:/nrlnanny/
   #scp nrlnanny.yaml root@$i:
   ssh root@$i "cd /nrlnanny; mv nrlnanny nrlnanny.$time ; cp /root/nrlnanny . ; systemctl restart nrlnanny "
 
done

