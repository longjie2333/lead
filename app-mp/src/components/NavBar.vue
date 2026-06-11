<template>
	<view
			class="nav-bar fixed left-0 right-0 bg-[rgb(247_247_247_/_89%)] z-50"
	>
		<view
				class="nav-status-bar"
				:style="{ 'height': statusBarHeight + 'px' }"
		/>

		<view
				class="nav-bar-main flex items-stretch"
				:class="mainClass"
				:style="{ 'height': navBarHeight + 'px', 'margin': `0 ${(menuButtonRect.width + menuButtonRect.gap * 2)}px 0 ${ mainClass !== '' ? 0 : menuButtonRect.gap}px` }"
		>
			<view
					class="nav-bar-menu flex items-stretch gap-2"
			>
				<view
						class="nav-bar-tools px-2.5 rounded-xl flex items-center justify-center bg-white"
						v-if="!isFirstPage"
						@click="handleBack"
				>
					<view class="i-lucide-chevron-left text-2xl" />
				</view>
				<view
						class="user-card rounded-xl p-1 px-4 flex gap-2 items-center bg-white"
						v-if="isFirstPage && showUserCard"
						@click="toManagePage"
				>
					<view
							class="user-card-avatar text-2xl i-lucide-user-cog mt-[3px]"
							:style="{ display: userStore.user.isAdmin ? 'inline-block' : 'none' }"
					/>
					<view
							class="user-card-avatar text-xl i-lucide-user mt-[3px]"
							:style="{ display: userStore.user.isAdmin ? 'none' : 'inline-block' }"
					/>
					<view
							class="user-card-info"
					>
						<view
								class="truncate text-sm text-ink font-semibold"
						>
							{{ userStore.user.username }}
						</view>
						<view
								class="text-[10px] text-gray-600"
						>
							{{ roleLabel(userStore.user) }}
						</view>
					</view>
				</view>
				<slot></slot>
			</view>
		</view>
	</view>
	<view
			class="nav-bar-placeholder"
			:style="{ 'height': height + 'px' }"
	/>
</template>

<script>
import { useUserStore } from '../stores/user'
import { mapStores } from 'pinia'
import { roleLabel } from '../utils'

export default {
	props: {
		title: {
			type: String,
			default: ''
		},
		showUserCard: {
			type: Boolean,
			default: true
		},
		mainClass: {
			type: String,
			default: ''
		}
	},
	data() {
		return {
			height: 0,
			statusBarHeight: 0,
			navBarHeight: 0,
			menuButtonRect: {
				width: 0,
				height: 0,
				top: 0,
				left: 0,
				right: 0,
				bottom: 0,
				gap: 0
			},
			isFirstPage: true,
		}
	},
	created() {
		this.getHeight()
		this.getPageInfo()
	},
	computed: {
		...mapStores(useUserStore)
	},
	methods: {
		roleLabel,
		getHeight() {
			if (uni.canIUse('getMenuButtonBoundingClientRect')) {
				const sysInfo = uni.getSystemInfoSync()
				const rect = uni.getMenuButtonBoundingClientRect()
				const navBarHeight = (rect.top - sysInfo.statusBarHeight) * 2 + rect.height

				this.height = sysInfo.statusBarHeight + navBarHeight
				this.statusBarHeight = sysInfo.statusBarHeight
				this.navBarHeight = navBarHeight
				this.menuButtonRect = rect
				this.menuButtonRect.gap = sysInfo.screenWidth - rect.right
			} else {
				uni.showToast({
					title: '您的微信版本过低，界面可能会显示不正常',
					icon: 'none',
					duration: 4000
				})
			}
		},
		getPageInfo() {
			const pages = getCurrentPages()
			this.isFirstPage = pages.length === 1
		},
		handleBack() {
			if (this.isFirstPage) return

			uni.navigateBack({
				delta: 1
			})
		},
		async toManagePage() {
			uni.navigateTo({
				url: '/pages/manage/manage'
			})
		}
	}
}
</script>

<style scoped>

</style>