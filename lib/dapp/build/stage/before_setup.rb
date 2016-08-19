module Dapp
  module Build
    module Stage
      # BeforeSetup
      class BeforeSetup < Base
        def initialize(application, next_stage)
          @prev_stage = InstallGroup::GAPostPatch.new(application, self)
          super
        end

        def empty?
          super && !application.builder.before_setup?
        end

        def dependencies
          prev_stage.prev_stage.dependencies
        end

        def image
          super do |image|
            application.builder.before_setup(image)
          end
        end
      end # BeforeSetup
    end # Stage
  end # Build
end # Dapp
