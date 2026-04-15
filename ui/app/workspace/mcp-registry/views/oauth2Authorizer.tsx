"use client"

import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard"
import { getErrorMessage } from "@/lib/store/apis/baseApi";
import { useCompleteOAuthFlowMutation, useLazyGetOAuthConfigStatusQuery } from "@/lib/store/apis/mcpApi";
import { Copy, ExternalLink, Link2, Loader2 } from "lucide-react"
import { useCallback, useEffect, useRef, useState } from "react"

interface OAuth2AuthorizerProps {
	open: boolean
	onClose: () => void
	onSuccess: () => void
	onError: (error: string) => void
	authorizeUrl: string
	oauthConfigId: string
	mcpClientId: string
	statusUrl?: string
	completeUrl?: string
	nextSteps?: string[]
}

export const OAuth2Authorizer: React.FC<OAuth2AuthorizerProps> = ({
	open,
	onClose,
	onSuccess,
	onError,
	authorizeUrl,
	oauthConfigId,
	mcpClientId,
	statusUrl,
	completeUrl,
	nextSteps,
}) => {
	const [status, setStatus] = useState<"pending" | "polling" | "success" | "failed">("pending")
	const [errorMessage, setErrorMessage] = useState<string | null>(null)
	const popupRef = useRef<Window | null>(null)
	const pollIntervalRef = useRef<NodeJS.Timeout | null>(null)
	const { copy } = useCopyToClipboard()

	// RTK Query hooks
	const [getOAuthStatus] = useLazyGetOAuthConfigStatusQuery()
	const [completeOAuth] = useCompleteOAuthFlowMutation()

	// Stop polling
	const stopPolling = useCallback(() => {
		if (pollIntervalRef.current) {
			clearInterval(pollIntervalRef.current)
			pollIntervalRef.current = null
		}
	}, [])

	// Handle successful OAuth completion
	const handleOAuthComplete = useCallback(async () => {
		// Close popup if still open
		if (popupRef.current && !popupRef.current.closed) {
			popupRef.current.close()
		}

		// Call complete-oauth endpoint using RTK Query mutation
		// Use oauthConfigId instead of mcpClientId for multi-instance support
		try {
			await completeOAuth(oauthConfigId).unwrap()
			setStatus("success")
			onSuccess()
			setTimeout(() => {
				onClose()
			}, 1000)
		} catch (error) {
			const errMsg = getErrorMessage(error)
			setStatus("failed")
			setErrorMessage(errMsg)
			onError(errMsg)
		}
	}, [oauthConfigId, completeOAuth, onSuccess, onClose, onError])

	// Handle OAuth failure
	const handleOAuthFailed = useCallback((reason: string) => {
		stopPolling()
		if (popupRef.current && !popupRef.current.closed) {
			popupRef.current.close()
		}
		setStatus("failed")
		setErrorMessage(reason)
		onError(reason)
	}, [stopPolling, onError])

	// Check OAuth status (called by postMessage or polling)
	const checkOAuthStatus = useCallback(async () => {
		try {
			const result = await getOAuthStatus(oauthConfigId).unwrap()

			if (result.status === "authorized") {
				stopPolling()
				await handleOAuthComplete()
			} else if (result.status === "failed" || result.status === "expired") {
				handleOAuthFailed(`Authorization ${result.status}`)
			}
		} catch (error) {
			console.error("Error checking OAuth status:", error)
		}
	}, [oauthConfigId, getOAuthStatus, stopPolling, handleOAuthComplete, handleOAuthFailed])

	// Poll OAuth status
	const startPolling = useCallback(() => {
		// Clear any existing interval
		if (pollIntervalRef.current) {
			clearInterval(pollIntervalRef.current)
		}

		pollIntervalRef.current = setInterval(async () => {
			// Check if popup is still open
			if (popupRef.current && popupRef.current.closed) {
				// Popup closed - check status before assuming cancellation
				// (OAuth callback page closes the popup after success)
				try {
					const result = await getOAuthStatus(oauthConfigId).unwrap()
					if (result.status === "authorized") {
						stopPolling()
						await handleOAuthComplete()
					} else if (result.status === "failed" || result.status === "expired") {
						stopPolling()
						handleOAuthFailed("Authorization failed")
					}
					// pending or other non-terminal: let polling continue
				} catch {
					// transient fetch error: let polling continue
				}
				return
			}

			await checkOAuthStatus()
		}, 2000) // Poll every 2 seconds
	}, [checkOAuthStatus, handleOAuthFailed])

	// Open popup and start polling
	const openPopup = useCallback(() => {
		// Close any existing popup
		if (popupRef.current && !popupRef.current.closed) {
			popupRef.current.close()
		}

		// Open OAuth popup
		const width = 600
		const height = 700
		const left = window.screen.width / 2 - width / 2
		const top = window.screen.height / 2 - height / 2

		popupRef.current = window.open(
			authorizeUrl,
			"oauth_popup",
			`width=${width},height=${height},left=${left},top=${top},resizable=yes,scrollbars=yes`,
		)

		setStatus("polling")

		// Start polling OAuth status
		startPolling()
	}, [authorizeUrl, startPolling])

	// Listen for postMessage from OAuth callback popup
	useEffect(() => {
		const handleMessage = (event: MessageEvent) => {
			// Verify message is from OAuth callback
			if (event.data?.type === "oauth_success") {
				// Trigger immediate status check; stopPolling is called inside
				// checkOAuthStatus only after a confirmed terminal state, so
				// transient fetch errors still allow polling to continue.
				checkOAuthStatus()
			}
		}

		window.addEventListener("message", handleMessage)
		return () => {
			window.removeEventListener("message", handleMessage)
		}
	}, [checkOAuthStatus])

	// Open popup when dialog opens
	useEffect(() => {
		if (open && status === "pending") {
			openPopup()
		}
	}, [open, status, openPopup])

	// Cleanup on unmount
	useEffect(() => {
		return () => {
			stopPolling()
			if (popupRef.current && !popupRef.current.closed) {
				popupRef.current.close()
			}
		}
	}, [stopPolling])

	const handleRetry = () => {
		setStatus("pending")
		setErrorMessage(null)
		openPopup()
	}

	const handleCancel = () => {
		stopPolling()
		if (popupRef.current && !popupRef.current.closed) {
			popupRef.current.close()
		}
		onClose()
	}

	return (
		<Dialog open={open}>
			<DialogContent className="sm:max-w-md" onPointerDownOutside={(e) => e.preventDefault()} onEscapeKeyDown={(e) => e.preventDefault()}>
				<DialogHeader>
					<DialogTitle>OAuth Authorization</DialogTitle>
					<DialogDescription>
						{status === "pending" && "Opening authorization window..."}
						{status === "polling" && "Waiting for authorization..."}
						{status === "success" && "Authorization successful!"}
						{status === "failed" && "Authorization failed"}
					</DialogDescription>
				</DialogHeader>

				<div className="flex flex-col items-center justify-center space-y-4">
					{status === "polling" && (
						<>
							<Loader2 className="text-secondary-foreground h-4 w-4 animate-spin" />
							<p className="text-muted-foreground text-sm">Please complete authorization in the popup window</p>
						</>
					)}

					{status === "success" && (
						<>
							<div className="flex h-12 w-12 items-center justify-center rounded-full bg-green-100">
								<svg className="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
								</svg>
							</div>
							<p className="text-sm text-green-600">MCP server connected successfully!</p>
						</>
					)}

					{status === "failed" && (
						<>
							<div className="flex h-12 w-12 items-center justify-center rounded-full bg-red-100">
								<svg className="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
									<path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
								</svg>
							</div>
							<p className="text-sm text-red-600">{errorMessage || "An error occurred"}</p>
							<Button onClick={handleRetry} variant="outline">
								Retry
							</Button>
						</>
					)}
				</div>

				<div className="bg-muted/30 space-y-3 rounded-md border p-3">
					<div className="space-y-1">
						<p className="text-xs font-medium tracking-wide uppercase">OAuth Context</p>
						<p className="font-mono text-xs break-all">oauth_config_id: {oauthConfigId}</p>
						<p className="font-mono text-xs break-all">mcp_client_id: {mcpClientId}</p>
					</div>

					{nextSteps && nextSteps.length > 0 && (
						<div className="space-y-1">
							<p className="text-xs font-medium tracking-wide uppercase">Next Steps</p>
							<ol className="text-muted-foreground list-decimal space-y-1 pl-4 text-xs">
								{nextSteps.map((step) => (
									<li key={step}>{step}</li>
								))}
							</ol>
						</div>
					)}

					<div className="flex flex-wrap gap-2">
						<Button onClick={openPopup} variant="outline" size="sm" dataTestId="oauth-authorizer-open-authorization">
							<ExternalLink className="h-4 w-4" />
							Open Authorization
						</Button>
						{statusUrl && (
							<Button onClick={() => void copy(statusUrl)} variant="outline" size="sm" dataTestId="oauth-authorizer-copy-status-url">
								<Link2 className="h-4 w-4" />
								Copy Status URL
							</Button>
						)}
						{completeUrl && (
							<Button onClick={() => void copy(completeUrl)} variant="outline" size="sm" dataTestId="oauth-authorizer-copy-complete-url">
								<Copy className="h-4 w-4" />
								Copy Complete URL
							</Button>
						)}
					</div>
				</div>

				{status === "polling" && (
					<DialogFooter>
						<Button onClick={handleCancel} variant="outline">
							Cancel
						</Button>
					</DialogFooter>
				)}
			</DialogContent>
		</Dialog>
	)
}
