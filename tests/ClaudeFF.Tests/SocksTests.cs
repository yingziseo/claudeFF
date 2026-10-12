using System.Net;
using System.Net.Sockets;
using System.Text;
using ClaudeFF.Core;
static class SocksTests
{
    public static async Task Run()
    {
        using var deadline=new CancellationTokenSource(TimeSpan.FromSeconds(10));var ct=deadline.Token;
        var listener=new TcpListener(IPAddress.Loopback,0);listener.Start();
        try
        {
            var endpoint=(IPEndPoint)listener.LocalEndpoint;
            using var client=new TcpClient();await client.ConnectAsync(endpoint.Address,endpoint.Port,ct);
            var server=await listener.AcceptTcpClientAsync(ct);
            var bridge=new SocksBridge("127.0.0.1",1,"","");var serving=bridge.Handle(server,ct);
            var stream=client.GetStream();await stream.WriteAsync(new byte[]{5,1,0},ct);
            var method=await SocksBridge.Read(stream,2,ct);if(!method.SequenceEqual(new byte[]{5,0}))throw new Exception("SOCKS negotiation");
            var port=SocksBridge.CanaryPort;
            await stream.WriteAsync(new byte[]{5,1,0,1,192,0,2,123,(byte)(port>>8),(byte)(port&255)},ct);
            var response=await SocksBridge.Read(stream,10,ct);if(response[1]!=0)throw new Exception("Canary rejected");
            var nonce=Guid.NewGuid().ToString("N");await stream.WriteAsync(Encoding.ASCII.GetBytes(nonce),ct);
            var echoed=Encoding.ASCII.GetString(await SocksBridge.Read(stream,47,ct));if(echoed!="claudeFF-relay:"+nonce)throw new Exception("Challenge mismatch");
            await serving;
            Console.WriteLine("PASS SOCKS TCP 实际握手与随机挑战回显");
        }
        finally{listener.Stop();}
        var relayListener=new TcpListener(IPAddress.Loopback,0);relayListener.Start();
        var unavailable=new TcpListener(IPAddress.Loopback,0);unavailable.Start();int closedPort=((IPEndPoint)unavailable.LocalEndpoint).Port;unavailable.Stop();
        try
        {
            using var client=new TcpClient();await client.ConnectAsync(IPAddress.Loopback,((IPEndPoint)relayListener.LocalEndpoint).Port,ct);
            var accepted=await relayListener.AcceptTcpClientAsync(ct);var serve=new SocksBridge("127.0.0.1",closedPort,"","").Handle(accepted,ct);
            var stream=client.GetStream();await stream.WriteAsync(new byte[]{5,1,0},ct);await SocksBridge.Read(stream,2,ct);
            await stream.WriteAsync(new byte[]{5,1,0,1,192,0,2,200,0,80},ct);
            var rejection=await SocksBridge.Read(stream,10,ct);if(rejection[1]==0)throw new Exception("Upstream failure accepted");
            await serve;Console.WriteLine("PASS 上游代理连接失败向客户端返回失败");
        }
        finally{relayListener.Stop();}
        await AuthenticatedForwarding();
    }
    private static async Task AuthenticatedForwarding()
    {
        using var deadline=new CancellationTokenSource(TimeSpan.FromSeconds(10));var ct=deadline.Token;
        var upstreamListener=new TcpListener(IPAddress.Loopback,0);upstreamListener.Start();
        var bridgeListener=new TcpListener(IPAddress.Loopback,0);bridgeListener.Start();
        try
        {
            var upstream=Task.Run(async()=>
            {
                using var peer=await upstreamListener.AcceptTcpClientAsync(ct);var stream=peer.GetStream();
                var auth=await SocksBridge.Read(stream,3,ct);if(!auth.SequenceEqual(new byte[]{5,1,2}))throw new Exception("Auth negotiation mismatch");
                await stream.WriteAsync(new byte[]{5,2},ct);
                var userHead=await SocksBridge.Read(stream,2,ct);var user=Encoding.UTF8.GetString(await SocksBridge.Read(stream,userHead[1],ct));
                var passHead=await SocksBridge.Read(stream,1,ct);var pass=Encoding.UTF8.GetString(await SocksBridge.Read(stream,passHead[0],ct));
                if(user!="test-user"||pass!="test-password")throw new Exception("Credential forwarding mismatch");
                await stream.WriteAsync(new byte[]{1,0},ct);
                var request=await SocksBridge.Read(stream,10,ct);if(request[1]!=1)throw new Exception("CONNECT missing");
                await stream.WriteAsync(new byte[]{5,0,0,1,127,0,0,1,0,0},ct);
                var message=await SocksBridge.Read(stream,5,ct);await stream.WriteAsync(message,ct);
            },ct);
            using var client=new TcpClient();await client.ConnectAsync(IPAddress.Loopback,((IPEndPoint)bridgeListener.LocalEndpoint).Port,ct);
            var accepted=await bridgeListener.AcceptTcpClientAsync(ct);
            var serve=new SocksBridge("127.0.0.1",((IPEndPoint)upstreamListener.LocalEndpoint).Port,"test-user","test-password").Handle(accepted,ct);
            var channel=client.GetStream();await channel.WriteAsync(new byte[]{5,1,0},ct);await SocksBridge.Read(channel,2,ct);
            await channel.WriteAsync(new byte[]{5,1,0,1,192,0,2,200,0,80},ct);
            var answer=await SocksBridge.Read(channel,10,ct);if(answer[1]!=0)throw new Exception("CONNECT failed");
            await channel.WriteAsync(Encoding.ASCII.GetBytes("hello"),ct);client.Client.Shutdown(SocketShutdown.Send);
            if(Encoding.ASCII.GetString(await SocksBridge.Read(channel,5,ct))!="hello")throw new Exception("Payload mismatch");
            await Task.WhenAll(serve,upstream);Console.WriteLine("PASS SOCKS 上游认证、双向 TCP 数据转发和半关闭");
        }
        finally{upstreamListener.Stop();bridgeListener.Stop();}
    }

}
