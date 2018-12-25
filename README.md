# cvsxfertool
A simple remote network bridge

The purpose of this project is to provide a fake ethernet adapter on my travel laptop (using tuntap) and make it think it is
on my LAN at work.

I am using ssh port forwarding to encrypt the traffic on the internet, and my setup looks like this:

                         +--------+                  +------+                  +------+
                         | LAPTOP | -- ssh tunnel -- | HOME | -- ssh tunnel -- | WORK |
                         +--------+                  +------+                  +------+
 
Laptop sets up the forward tunnel, and work sets up a reverse tunnel.  Home is a static IP, laptop and work may be behind a
 firewall.
 
Internally on LAPTOP, this program takes the data from the tap interface file and puts it into the tunnel.  On the other end,
this program takes data from the tunnel and gives it to the tap interface which is bridged to the lan.  I expect DHCP to run
over the system.

Usage:

      On the work side:
      
      # modprobe tun
      # ssh -N -R 127.0.0.1:<port>:127.0.0.1:<port> root@HOME
      # cvsxfertool &
      # bridge addbr br0
      # brctl addif br0 tap0
      # brctl addif br0 eth0:1

      On the laptop side:
  
      $ modprobe tun
      $ cvsxfertool home &
      $ dhclient tap0
      
Note: This is initial and untested... Updates will follow

