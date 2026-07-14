import React, { useEffect, useMemo, useState } from 'react'
import {
  api,
  API,
  useAlert,
  Page,
  ListHeader,
  Card,
  SectionHeader,
  StatTile,
  KeyVal,
  StatusDot,
  Toggle,
  TextField,
  ModalConfirm,
  Loading,
  Box,
  Button,
  ButtonIcon,
  ButtonText,
  Heading,
  HStack,
  Icon,
  Pressable,
  Text,
  VStack,
  ChevronDownIcon,
  ChevronRightIcon,
  CopyIcon,
  GlobeIcon
} from '@spr-networks/plugin-ui'

class UsqueAPI extends API {
  constructor() {
    super(`plugins/${api.pluginURI() || 'spr-usque'}/`)
  }
  status() {
    return this.get('status')
  }
  config() {
    return this.get('config')
  }
  saveConfig(value) {
    return this.put('config', value)
  }
  register(value) {
    return this.post('register', value)
  }
  setTunnel(running) {
    return this.put('tunnel', { Running: running })
  }
  restart() {
    return this.post('restart', {})
  }
  trace() {
    return this.get('trace')
  }
}

const usque = new UsqueAPI()

const copyText = (value, alert, label = 'Value') => {
  if (!value) return
  try {
    navigator.clipboard
      .writeText(value)
      .then(() => alert.success(`${label} copied`))
      .catch(() => alert.error('Copy failed'))
  } catch (err) {
    alert.error('Copy failed')
  }
}

const CopyButton = ({ value, label }) => {
  const alert = useAlert()
  return (
    <Button
      size="xs"
      variant="outline"
      action="secondary"
      isDisabled={!value}
      onPress={() => copyText(value, alert, label)}
    >
      <ButtonIcon as={CopyIcon} mr="$1" />
      <ButtonText>Copy</ButtonText>
    </Button>
  )
}

const Step = ({ number, title, children }) => (
  <HStack space="md" alignItems="flex-start">
    <Box
      w={26}
      h={26}
      flexShrink={0}
      borderRadius="$full"
      alignItems="center"
      justifyContent="center"
      bg="$primary100"
      sx={{ _dark: { bg: '$primary800' } }}
    >
      <Text
        size="2xs"
        fontWeight="$bold"
        color="$primary800"
        sx={{ _dark: { color: '$primary100' } }}
      >
        {number}
      </Text>
    </Box>
    <VStack space="xs" flex={1}>
      <Text
        size="sm"
        fontWeight="$semibold"
        color="$textLight900"
        sx={{ _dark: { color: '$textDark50' } }}
      >
        {title}
      </Text>
      <Text size="sm" color="$muted500" lineHeight="$sm">
        {children}
      </Text>
    </VStack>
  </HStack>
)

const Disclosure = ({ open, onToggle, label, children }) => (
  <VStack space="md">
    <Pressable onPress={onToggle}>
      <HStack space="xs" alignItems="center">
        <Icon
          as={open ? ChevronDownIcon : ChevronRightIcon}
          size="sm"
          color="$muted500"
        />
        <Text size="sm" color="$muted500" fontWeight="$medium">
          {label}
        </Text>
      </HStack>
    </Pressable>
    {open ? children : null}
  </VStack>
)

const MonoBlock = ({ children }) => (
  <Box
    borderRadius="$lg"
    borderWidth={1}
    borderColor="$muted200"
    bg="$backgroundContentLight"
    p="$3"
    sx={{
      _dark: { bg: '$backgroundContentDark', borderColor: '$borderColorCardDark' }
    }}
  >
    <Text
      size="xs"
      color="$textLight900"
      sx={{
        _dark: { color: '$textDark100' },
        '@base': {
          fontFamily: 'monospace',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all'
        }
      }}
    >
      {children}
    </Text>
  </Box>
)

const PathNode = ({ eyebrow, title, detail, online, warn }) => (
  <Box
    minWidth={142}
    flex={1}
    px="$3.5"
    py="$3"
    borderRadius="$xl"
    borderWidth={1}
    borderColor="$borderColorCardLight"
    bg="$backgroundContentLight"
    sx={{
      _dark: {
        bg: '$backgroundContentDark',
        borderColor: '$borderColorCardDark'
      }
    }}
  >
    <VStack space="xs">
      <HStack alignItems="center" space="xs">
        {online !== undefined ? <StatusDot online={online} warn={warn} size={8} /> : null}
        <Text
          size="2xs"
          color="$muted500"
          fontWeight="$bold"
          sx={{ '@base': { textTransform: 'uppercase', letterSpacing: 0.7 } }}
        >
          {eyebrow}
        </Text>
      </HStack>
      <Text
        size="sm"
        fontWeight="$semibold"
        color="$textLight900"
        sx={{ _dark: { color: '$textDark50' } }}
      >
        {title}
      </Text>
      <Text
        size="xs"
        color="$muted500"
        sx={{ '@base': { fontFamily: 'monospace' } }}
      >
        {detail || '—'}
      </Text>
    </VStack>
  </Box>
)

const Arrow = () => (
  <Text size="lg" color="$muted400" flexShrink={0}>
    →
  </Text>
)

const portError = (value) => {
  const text = String(value).trim()
  if (!/^\d+$/.test(text)) return 'Enter a port number'
  const number = parseInt(text, 10)
  return number < 1 || number > 65535 ? 'Port must be between 1 and 65535' : null
}

const deviceNameError = (value) => {
  const text = value.trim()
  if (!text) return null
  return /^[A-Za-z0-9._-]{1,64}$/.test(text)
    ? null
    : 'Letters, digits, dot, dash and underscore only (max 64)'
}

export default function Plugin() {
  const alert = useAlert()
  const [status, setStatus] = useState(null)
  const [statusError, setStatusError] = useState(null)
  const [loading, setLoading] = useState(true)

  const [savedConfig, setSavedConfig] = useState(null)
  const [connectPort, setConnectPort] = useState('443')
  const [endpointV6, setEndpointV6] = useState(false)
  const [http2, setHTTP2] = useState(false)
  const [tunnelIPv6, setTunnelIPv6] = useState(true)
  const [autoStart, setAutoStart] = useState(true)
  const [saving, setSaving] = useState(false)

  const [deviceName, setDeviceName] = useState('spr-usque')
  const [jwt, setJWT] = useState('')
  const [zeroTrustOpen, setZeroTrustOpen] = useState(false)
  const [registering, setRegistering] = useState(false)
  const [showReRegister, setShowReRegister] = useState(false)

  const [tunnelBusy, setTunnelBusy] = useState(null)
  const [showStop, setShowStop] = useState(false)
  const [tracing, setTracing] = useState(false)
  const [traceOutput, setTraceOutput] = useState(null)

  const adoptConfig = (config) => {
    const cfg = config || {}
    setSavedConfig(cfg)
    setConnectPort(String(cfg.ConnectPort ?? 443))
    setEndpointV6(cfg.EndpointVersion === 'v6')
    setHTTP2(cfg.Transport === 'http2')
    setTunnelIPv6(cfg.TunnelIPv6 !== false)
    setAutoStart(cfg.AutoStart !== false)
    setDeviceName(cfg.DeviceName || 'spr-usque')
  }

  const refreshStatus = () =>
    usque
      .status()
      .then((next) => {
        setStatus(next)
        setStatusError(null)
      })
      .catch((err) => setStatusError(err))
      .finally(() => setLoading(false))

  const load = () => {
    setLoading(true)
    Promise.all([usque.status(), usque.config()])
      .then(([nextStatus, config]) => {
        const cfg = config || {}
        setStatus(nextStatus)
        setSavedConfig(cfg)
        setConnectPort(String(cfg.ConnectPort ?? 443))
        setEndpointV6(cfg.EndpointVersion === 'v6')
        setHTTP2(cfg.Transport === 'http2')
        setTunnelIPv6(cfg.TunnelIPv6 !== false)
        setAutoStart(cfg.AutoStart !== false)
        setDeviceName(cfg.DeviceName || 'spr-usque')
        setStatusError(null)
      })
      .catch((err) => setStatusError(err))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    Promise.all([usque.status(), usque.config()])
      .then(([nextStatus, config]) => {
        setStatus(nextStatus)
        adoptConfig(config)
        setStatusError(null)
      })
      .catch((err) => setStatusError(err))
      .finally(() => setLoading(false))

    const timer = setInterval(() => {
      usque
        .status()
        .then((next) => {
          setStatus(next)
          setStatusError(null)
        })
        .catch((err) => setStatusError(err))
    }, 12000)
    return () => clearInterval(timer)
  }, [])

  const registered = !!status?.Registered
  const processRunning = !!status?.ProcessRunning
  const connected = !!status?.Connected
  const connectivity = status?.Connectivity || {}
  const verified = connected && !!connectivity.OK
  const gatewayIP = status?.GatewayIP || '172.30.118.2'
  const tunnelInterface = status?.TunnelInterface || 'warp0'
  const connectError = portError(connectPort)
  const nameError = deviceNameError(deviceName)

  const formConfig = useMemo(
    () => ({
      EndpointVersion: endpointV6 ? 'v6' : 'v4',
      ConnectPort: parseInt(connectPort, 10),
      Transport: http2 ? 'http2' : 'http3',
      TunnelIPv6: tunnelIPv6,
      AutoStart: autoStart,
      DeviceName: savedConfig?.DeviceName || deviceName.trim() || 'spr-usque'
    }),
    [endpointV6, connectPort, http2, tunnelIPv6, autoStart, savedConfig, deviceName]
  )

  const dirty =
    !!savedConfig &&
    (savedConfig.EndpointVersion !== formConfig.EndpointVersion ||
      Number(savedConfig.ConnectPort) !== formConfig.ConnectPort ||
      savedConfig.Transport !== formConfig.Transport ||
      savedConfig.TunnelIPv6 !== formConfig.TunnelIPv6 ||
      savedConfig.AutoStart !== formConfig.AutoStart)

  const register = (force) => {
    setRegistering(true)
    usque
      .register({ DeviceName: deviceName.trim(), JWT: jwt.trim(), Force: !!force })
      .then(() => {
        alert.success('Cloudflare WARP enrollment complete')
        setJWT('')
        setTimeout(load, 1000)
      })
      .catch((err) => alert.error('Enrollment failed', err))
      .finally(() => setRegistering(false))
  }

  const save = () => {
    setSaving(true)
    usque
      .saveConfig(formConfig)
      .then((config) => {
        adoptConfig(config || formConfig)
        alert.success(processRunning ? 'Settings saved — tunnel restarted' : 'Settings saved')
        setTimeout(refreshStatus, 800)
      })
      .catch((err) => alert.error('Could not save settings', err))
      .finally(() => setSaving(false))
  }

  const setTunnel = (running) => {
    setTunnelBusy(running ? 'start' : 'stop')
    usque
      .setTunnel(running)
      .then(() => {
        alert.success(running ? 'WARP tunnel starting' : 'WARP tunnel stopped')
        setTimeout(refreshStatus, running ? 1200 : 400)
      })
      .catch((err) => alert.error('Tunnel command failed', err))
      .finally(() => setTunnelBusy(null))
  }

  const restart = () => {
    setTunnelBusy('restart')
    usque
      .restart()
      .then(() => {
        alert.success('WARP tunnel restarting')
        setTimeout(refreshStatus, 1200)
      })
      .catch((err) => alert.error('Restart failed', err))
      .finally(() => setTunnelBusy(null))
  }

  const runTrace = () => {
    setTracing(true)
    usque
      .trace()
      .then((result) =>
        setTraceOutput(typeof result === 'string' ? result : JSON.stringify(result, null, 2))
      )
      .catch((err) => alert.error('WARP trace failed', err))
      .finally(() => setTracing(false))
  }

  let headerStatus = 'Not enrolled'
  let headerAction = 'muted'
  if (registered && verified) {
    headerStatus = 'Connected'
    headerAction = 'success'
  } else if (registered && processRunning) {
    headerStatus = connected ? 'Verifying' : 'Connecting'
    headerAction = 'warning'
  } else if (registered) {
    headerStatus = 'Stopped'
    headerAction = 'error'
  }

  const header = (
    <ListHeader
      title="WARP gateway"
      description="Linux TUN forwarding through Cloudflare WARP over MASQUE"
      mark="uq"
      status={loading || (statusError && !status) ? undefined : headerStatus}
      statusAction={headerAction}
    >
      {!loading && status ? (
        <Button size="sm" variant="outline" action="secondary" onPress={refreshStatus}>
          <ButtonText>Refresh</ButtonText>
        </Button>
      ) : null}
    </ListHeader>
  )

  if (loading) {
    return (
      <Page>
        {header}
        <Loading text="Contacting spr-usque…" />
      </Page>
    )
  }

  if (statusError && !status) {
    return (
      <Page>
        {header}
        <Card>
          <VStack space="md" alignItems="center" py="$6">
            <Heading size="sm" color="$textLight900" sx={{ _dark: { color: '$textDark50' } }}>
              Gateway backend unavailable
            </Heading>
            <Text size="sm" color="$muted500" textAlign="center" maxWidth={440}>
              The spr-usque container may still be starting. Confirm the plugin is enabled,
              /dev/net/tun is available on the Linux host, then retry.
            </Text>
            <Button size="sm" onPress={load}>
              <ButtonText>Retry</ButtonText>
            </Button>
          </VStack>
        </Card>
      </Page>
    )
  }

  if (!registered) {
    return (
      <Page>
        {header}

        <Card>
          <VStack space="lg">
            <HStack space="md" alignItems="center">
              <Box
                w={48}
                h={48}
                borderRadius="$xl"
                bg="$primary100"
                alignItems="center"
                justifyContent="center"
                flexShrink={0}
                sx={{ _dark: { bg: '$primary800' } }}
              >
                <Icon
                  as={GlobeIcon}
                  color="$primary700"
                  size="xl"
                  sx={{ _dark: { color: '$primary200' } }}
                />
              </Box>
              <VStack space="xs" flexShrink={1}>
                <Heading size="md" color="$textLight900" sx={{ _dark: { color: '$textDark50' } }}>
                  Turn WARP into an SPR forwarding destination
                </Heading>
                <Text size="sm" color="$muted500" lineHeight="$sm">
                  Enroll once. spr-usque creates a Linux TUN interface and routes selected
                  SPR traffic through Cloudflare—without proxy settings on client devices.
                </Text>
              </VStack>
            </HStack>

            <HStack alignItems="center" flexWrap="wrap" gap="$2">
              <PathNode eyebrow="Source" title="SPR policy" detail="selected devices" />
              <Arrow />
              <PathNode eyebrow="Gateway" title={gatewayIP} detail="spr-usque" online={false} />
              <Arrow />
              <PathNode eyebrow="Tunnel" title="warp0" detail="fail-closed" online={false} />
              <Arrow />
              <PathNode eyebrow="Egress" title="Cloudflare WARP" detail="MASQUE" online={false} />
            </HStack>

            <Box
              p="$3.5"
              borderRadius="$xl"
              borderWidth={1}
              borderColor="$muted200"
              bg="$backgroundContentLight"
              sx={{
                _dark: { bg: '$backgroundContentDark', borderColor: '$borderColorCardDark' }
              }}
            >
              <Text size="xs" color="$muted500" lineHeight="$sm">
                Traffic is blocked—not sent to the normal WAN—until enrollment and the WARP
                tunnel are both healthy.
              </Text>
            </Box>
          </VStack>
        </Card>

        <Card>
          <SectionHeader title="Enroll Cloudflare WARP" />
          <VStack space="lg">
            <VStack space="md">
              <Step number="1" title="Name the WARP device">
                This is how the router appears in your Cloudflare account or Zero Trust dashboard.
              </Step>
              <Step number="2" title="Register">
                A free WARP account needs no credentials. Zero Trust enrollment accepts a team JWT.
              </Step>
              <Step number="3" title="Choose the forwarding destination">
                After the tunnel connects, SPR exposes Cloudflare WARP as an online route sink.
              </Step>
            </VStack>

            <VStack space="md" maxWidth={500} w="$full">
              <TextField
                label="Device name"
                value={deviceName}
                onChangeText={setDeviceName}
                placeholder="spr-usque"
                helper="Visible in Cloudflare; stored with the plugin settings"
                error={nameError}
              />
              <Disclosure
                open={zeroTrustOpen}
                onToggle={() => setZeroTrustOpen((value) => !value)}
                label="Zero Trust enrollment (optional)"
              >
                <TextField
                  label="Team enrollment token (JWT)"
                  value={jwt}
                  onChangeText={setJWT}
                  placeholder="Paste the team token"
                  helper="Leave empty for regular WARP"
                  secureTextEntry
                />
              </Disclosure>
              <Button
                size="md"
                alignSelf="flex-start"
                isDisabled={registering || !!nameError}
                onPress={() => register(false)}
              >
                <ButtonText>{registering ? 'Enrolling…' : 'Enroll WARP device'}</ButtonText>
              </Button>
              <Text size="2xs" color="$muted500">
                Enrollment accepts the Cloudflare WARP Terms of Service.
              </Text>
            </VStack>
          </VStack>
        </Card>
      </Page>
    )
  }

  const stateTitle = verified
    ? 'WARP egress verified'
    : connected
    ? 'Tunnel connected'
    : processRunning
    ? 'Establishing MASQUE tunnel'
    : 'Gateway stopped'
  const stateHint = verified
    ? `${connectivity.Colo || 'Cloudflare edge'} · exit ${connectivity.IP || '—'}`
    : connected && connectivity.Error
    ? `Interface is up; verification failed: ${connectivity.Error}`
    : processRunning
    ? 'warp0 is starting. Transit traffic remains fail-closed until connected.'
    : status?.LastError
    ? `No traffic is forwarded: ${status.LastError}`
    : 'No traffic is forwarded while the tunnel is stopped.'

  return (
    <Page>
      {header}

      <Card>
        <VStack space="lg">
          <HStack justifyContent="space-between" alignItems="center" flexWrap="wrap" gap="$3">
            <HStack space="md" alignItems="center" flexShrink={1}>
              <StatusDot online={verified} warn={processRunning && !verified} size={12} />
              <VStack space="xs" flexShrink={1}>
                <Heading size="md" color="$textLight900" sx={{ _dark: { color: '$textDark50' } }}>
                  {stateTitle}
                </Heading>
                <Text size="sm" color="$muted500" lineHeight="$sm">
                  {stateHint}
                </Text>
              </VStack>
            </HStack>

            <HStack space="sm" alignItems="center" flexWrap="wrap">
              {!processRunning ? (
                <Button
                  size="sm"
                  isDisabled={tunnelBusy !== null}
                  onPress={() => setTunnel(true)}
                >
                  <ButtonText>{tunnelBusy === 'start' ? 'Starting…' : 'Start tunnel'}</ButtonText>
                </Button>
              ) : (
                <>
                  <Button
                    size="sm"
                    variant="outline"
                    action="secondary"
                    isDisabled={tunnelBusy !== null}
                    onPress={restart}
                  >
                    <ButtonText>
                      {tunnelBusy === 'restart' ? 'Restarting…' : 'Restart'}
                    </ButtonText>
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    action="negative"
                    isDisabled={tunnelBusy !== null}
                    onPress={() => setShowStop(true)}
                  >
                    <ButtonText>{tunnelBusy === 'stop' ? 'Stopping…' : 'Stop'}</ButtonText>
                  </Button>
                </>
              )}
            </HStack>
          </HStack>

          <HStack flexWrap="wrap" gap="$2">
            <StatTile label="Gateway" value={gatewayIP} mono />
            <StatTile label="Tunnel" value={tunnelInterface} mono />
            <StatTile
              label="Transport"
              value={status?.Transport === 'http2' ? 'HTTP/2 · TCP' : 'HTTP/3 · QUIC'}
            />
            <StatTile label="Uptime" value={processRunning ? status?.Uptime : '—'} mono />
            <StatTile label="WARP address" value={status?.WarpIPv4} mono />
          </HStack>

          {statusError ? (
            <Text size="xs" color="$muted500">
              The last status refresh failed; the dashboard will retry automatically.
            </Text>
          ) : null}
        </VStack>
      </Card>

      <Card>
        <SectionHeader title="Forwarding path" />
        <VStack space="lg">
          <HStack alignItems="center" flexWrap="wrap" gap="$2">
            <PathNode eyebrow="SPR" title="Forwarding policy" detail="group: warp" />
            <Arrow />
            <PathNode
              eyebrow="Destination"
              title={gatewayIP}
              detail="spr-usque"
              online={processRunning}
              warn={processRunning && !connected}
            />
            <Arrow />
            <PathNode
              eyebrow="Native TUN"
              title={tunnelInterface}
              detail={`MTU ${status?.TunnelMTU || 1280}`}
              online={connected}
              warn={processRunning && !connected}
            />
            <Arrow />
            <PathNode
              eyebrow="WARP edge"
              title={connectivity.Colo || 'Cloudflare'}
              detail={connectivity.IP || status?.Endpoint || 'waiting'}
              online={verified}
              warn={connected && !verified}
            />
          </HStack>

          <Box
            p="$3.5"
            borderRadius="$xl"
            borderWidth={1}
            borderColor={verified ? '$success200' : '$muted200'}
            bg="$backgroundContentLight"
            sx={{
              _dark: {
                bg: '$backgroundContentDark',
                borderColor: verified ? '$success800' : '$borderColorCardDark'
              }
            }}
          >
            <HStack space="sm" alignItems="flex-start">
              <StatusDot online={verified} warn={processRunning && !verified} size={9} />
              <VStack space="xs" flex={1}>
                <Text
                  size="sm"
                  fontWeight="$semibold"
                  color="$textLight900"
                  sx={{ _dark: { color: '$textDark50' } }}
                >
                  {verified ? 'Forwarding destination is online' : 'Fail-closed protection is active'}
                </Text>
                <Text size="xs" color="$muted500" lineHeight="$sm">
                  {verified
                    ? 'Traffic policy-routed to this destination is SNATed to the enrolled WARP address and returned to the original SPR client.'
                    : 'The policy table ends in an unreachable route, so selected traffic cannot escape through the container’s ordinary WAN path.'}
                </Text>
              </VStack>
            </HStack>
          </Box>
        </VStack>
      </Card>

      <Card>
        <SectionHeader title="Use as an SPR destination" />
        <VStack space="lg">
          <HStack justifyContent="space-between" alignItems="center" flexWrap="wrap" gap="$2">
            <KeyVal label="Forwarding destination" value={`Cloudflare WARP · ${gatewayIP}`} mono />
            <CopyButton value={gatewayIP} label="Gateway IP" />
          </HStack>
          <KeyVal label="Custom interface" value="spr-usque" mono />
          <KeyVal label="Eligibility group" value="warp" mono />

          <VStack space="md">
            <Step number="1" title="Grant device eligibility">
              Add devices that may use this gateway to the warp group in SPR.
            </Step>
            <Step number="2" title="Create a forwarding rule">
              Select Cloudflare WARP as the destination for the devices, domains, or traffic you want tunneled.
            </Step>
            <Step number="3" title="Verify the route">
              The destination should show online above. Client applications need no SOCKS or HTTP proxy configuration.
            </Step>
          </VStack>
        </VStack>
      </Card>

      <Card>
        <SectionHeader title="Tunnel settings" />
        <VStack space="lg" maxWidth={560} w="$full">
          <TextField
            label="MASQUE connect port"
            value={connectPort}
            onChangeText={setConnectPort}
            placeholder="443"
            helper="Cloudflare endpoint port; 443 is recommended"
            error={connectError}
          />

          <HStack justifyContent="space-between" alignItems="center" gap="$4">
            <VStack space="xs" flexShrink={1}>
              <Text size="sm" fontWeight="$semibold" color="$textLight900" sx={{ _dark: { color: '$textDark100' } }}>
                HTTP/2 transport
              </Text>
              <Text size="xs" color="$muted500">
                Use TCP + TLS instead of the default HTTP/3 over QUIC.
              </Text>
            </VStack>
            <Toggle value={http2} onPress={() => setHTTP2(!http2)} label="HTTP/2 transport" />
          </HStack>

          <HStack justifyContent="space-between" alignItems="center" gap="$4">
            <VStack space="xs" flexShrink={1}>
              <Text size="sm" fontWeight="$semibold" color="$textLight900" sx={{ _dark: { color: '$textDark100' } }}>
                IPv6 endpoint underlay
              </Text>
              <Text size="xs" color="$muted500">
                Reach Cloudflare over IPv6. Requires working IPv6 on the SPR host.
              </Text>
            </VStack>
            <Toggle value={endpointV6} onPress={() => setEndpointV6(!endpointV6)} label="IPv6 endpoint underlay" />
          </HStack>

          <HStack justifyContent="space-between" alignItems="center" gap="$4">
            <VStack space="xs" flexShrink={1}>
              <Text size="sm" fontWeight="$semibold" color="$textLight900" sx={{ _dark: { color: '$textDark100' } }}>
                IPv6 inside WARP
              </Text>
              <Text size="xs" color="$muted500">
                Keep the WARP IPv6 address on warp0. IPv4 gateway forwarding remains enabled.
              </Text>
            </VStack>
            <Toggle value={tunnelIPv6} onPress={() => setTunnelIPv6(!tunnelIPv6)} label="IPv6 inside WARP" />
          </HStack>

          <HStack justifyContent="space-between" alignItems="center" gap="$4">
            <VStack space="xs" flexShrink={1}>
              <Text size="sm" fontWeight="$semibold" color="$textLight900" sx={{ _dark: { color: '$textDark100' } }}>
                Start automatically
              </Text>
              <Text size="xs" color="$muted500">
                Bring the tunnel up whenever the plugin container starts.
              </Text>
            </VStack>
            <Toggle value={autoStart} onPress={() => setAutoStart(!autoStart)} label="Start automatically" />
          </HStack>

          <HStack space="md" alignItems="center" flexWrap="wrap" gap="$2">
            <Button
              size="sm"
              isDisabled={!dirty || saving || !!connectError}
              onPress={save}
            >
              <ButtonText>{saving ? 'Applying…' : 'Apply changes'}</ButtonText>
            </Button>
            <Text size="xs" color="$muted500">
              {processRunning ? 'Applying changes restarts the tunnel.' : 'Changes apply on the next start.'}
            </Text>
          </HStack>
        </VStack>
      </Card>

      <Card>
        <SectionHeader title="Enrollment & diagnostics" />
        <VStack space="lg">
          <HStack flexWrap="wrap" gap="$2">
            <StatTile label="Device ID" value={status?.DeviceID} mono />
            <StatTile label="WARP IPv4" value={status?.WarpIPv4} mono />
            <StatTile label="WARP IPv6" value={status?.WarpIPv6} mono />
            <StatTile label="Endpoint" value={status?.Endpoint} mono />
          </HStack>

          <HStack justifyContent="space-between" alignItems="center" flexWrap="wrap" gap="$3">
            <VStack space="xs" flexShrink={1}>
              <Text size="sm" fontWeight="$semibold" color="$textLight900" sx={{ _dark: { color: '$textDark100' } }}>
                End-to-end WARP trace
              </Text>
              <Text size="xs" color="$muted500">
                DNS and HTTPS are bound to warp0, so this test cannot silently use the normal WAN.
              </Text>
            </VStack>
            <HStack space="sm">
              <Button
                size="xs"
                variant="outline"
                action="secondary"
                isDisabled={!connected || tracing}
                onPress={runTrace}
              >
                <ButtonText>{tracing ? 'Running…' : 'Run trace'}</ButtonText>
              </Button>
              <Button
                size="xs"
                variant="outline"
                action="negative"
                isDisabled={registering}
                onPress={() => setShowReRegister(true)}
              >
                <ButtonText>{registering ? 'Enrolling…' : 'Re-enroll'}</ButtonText>
              </Button>
            </HStack>
          </HStack>

          {traceOutput != null ? (
            <VStack space="sm">
              <HStack justifyContent="space-between" alignItems="center">
                <Text size="xs" color="$muted500">
                  cloudflare.com/cdn-cgi/trace via {tunnelInterface}
                </Text>
                <HStack space="sm">
                  <CopyButton value={traceOutput} label="Trace" />
                  <Button size="xs" variant="link" action="secondary" onPress={() => setTraceOutput(null)}>
                    <ButtonText>Hide</ButtonText>
                  </Button>
                </HStack>
              </HStack>
              <MonoBlock>{traceOutput.trim()}</MonoBlock>
            </VStack>
          ) : null}

          <Disclosure
            open={zeroTrustOpen}
            onToggle={() => setZeroTrustOpen((value) => !value)}
            label="Zero Trust token for next re-enrollment"
          >
            <VStack space="md" maxWidth={500}>
              <TextField
                label="Device name"
                value={deviceName}
                onChangeText={setDeviceName}
                error={nameError}
              />
              <TextField
                label="Team enrollment token (JWT)"
                value={jwt}
                onChangeText={setJWT}
                placeholder="Optional team token"
                secureTextEntry
              />
            </VStack>
          </Disclosure>

          <Text size="2xs" color="$muted500">
            WARP keys and access tokens are stored mode 0600 and are never returned to this UI.
          </Text>
        </VStack>
      </Card>

      <ModalConfirm
        isOpen={showStop}
        onClose={() => setShowStop(false)}
        onConfirm={() => {
          setShowStop(false)
          setTunnel(false)
        }}
        title="Stop the WARP tunnel?"
        message="Forwarded traffic will be blocked by the fail-closed gateway until the tunnel is started again."
        confirmText="Stop tunnel"
        destructive
      />

      <ModalConfirm
        isOpen={showReRegister}
        onClose={() => setShowReRegister(false)}
        onConfirm={() => {
          setShowReRegister(false)
          register(true)
        }}
        title="Re-enroll this WARP gateway?"
        message={`The tunnel will stop while a fresh Cloudflare device replaces ${status?.DeviceID || 'the current enrollment'}. Existing credentials are retained as config.json.bak.`}
        confirmText="Re-enroll"
        destructive
      />
    </Page>
  )
}
